import { computed, DestroyRef, inject, Injectable, InjectionToken, type Provider, signal } from '@angular/core';
import { catchError, defer, EMPTY, finalize, Observable, Subscription, tap, throwError } from 'rxjs';

import { hasAnyPermission, userId } from '@/core/services/auth/auth.selectors';
import { CRM_MODULE_PERMISSIONS } from '@/define';
import { CrmDashboardLayoutService, type DashboardLayoutView } from '@/core/services/crm/crm-dashboard-layout.service';
import { STORAGE_KEY_SELECT } from '@/define';
import { StorageService } from '@/share/services/storage.service';

import { allowedWidgets, CRM_DASHBOARD_WIDGETS, type DashboardWidgetDef, type DashboardZone } from './dashboard-widgets';
import { type CachedLayout, type LayoutStorage, ownerKey, readCachedLayout, writeCachedLayout } from './layout-cache';
import {
  type DashboardLayoutConfig, type DashboardLayoutItem, hideItem, layoutSignature, mergeLayout, showItem, systemDefaultLayout,
} from './layout.util';
import { DASHBOARD_LAYOUT_PRESETS, type DashboardPresetId, presetLayout } from './layout-presets';

/**
 * Chủ của bản tạm (`<uid>` — CRM 1 thương hiệu/DB nên không kèm công ty như ERP; null = chưa đăng nhập ⇒ không đọc/ghi). Token thay vì gọi thẳng
 * selector: spec provide được mà không cần `vi.mock` — `vi.mock` vô tác dụng dưới `ng test` (builder đã gộp module).
 *
 * KHÔNG `factory` mặc định: token có factory là singleton ROOT ⇒ tính 1 lần cả đời app, đăng nhập user khác trong cùng
 * tab (SPA) vẫn dùng khoá của user trước (QA story-10 P2). Provide bằng `provideDashboardLayoutOwner()` cạnh store.
 */
export const DASHBOARD_LAYOUT_OWNER = new InjectionToken<string | null>('DASHBOARD_LAYOUT_OWNER');

/** Tính owner MỖI lần dựng injector chứa nó (provide ở component Dashboard). */
export function provideDashboardLayoutOwner(): Provider {
  return { provide: DASHBOARD_LAYOUT_OWNER, useFactory: () => ownerKey(userId()) };
}

/**
 * Registry khối user ĐƯỢC THẤY (CRM gác quyền khối — ERP không có): khối thiếu quyền bị bỏ khỏi registry của trang ⇒
 * `mergeLayout` gạt key đó khỏi bố cục, Kho khối/mẫu bố cục không có nó, không dựng host ⇒ không tải chunk, không gọi
 * API. Token (cùng lý do `DASHBOARD_LAYOUT_OWNER`): spec provide registry đã lọc; KHÔNG factory mặc định (singleton root
 * ⇒ đăng nhập user khác cùng tab vẫn dùng quyền user trước). Quyền đã nạp xong trước khi vào trang (`authGuard` chờ
 * `authInitialized`) ⇒ tính 1 lần lúc dựng trang là đủ.
 */
export const DASHBOARD_WIDGET_REGISTRY = new InjectionToken<readonly DashboardWidgetDef[]>('DASHBOARD_WIDGET_REGISTRY');

/**
 * User được LƯU bố cục riêng: 3 route `/crm-dashboard/layout` (GET/PUT/DELETE) đòi viewAppointment|viewCustomer|
 * viewPlanOrder (BE route_perm_map). Thiếu ⇒ store KHÔNG gọi API bố cục (tránh 403), dùng mặc định hệ thống chỉ đọc;
 * trang ẩn Tuỳ chỉnh/Thêm khối/Mẫu bố cục/Ẩn khối.
 */
export const DASHBOARD_LAYOUT_PERSIST = new InjectionToken<boolean>('DASHBOARD_LAYOUT_PERSIST');

/** Registry đã lọc quyền + quyền lưu bố cục — tính MỖI lần dựng trang (provide ở component Dashboard). */
export function provideDashboardWidgetRegistry(): Provider[] {
  return [
    {
      provide: DASHBOARD_WIDGET_REGISTRY,
      useFactory: () => allowedWidgets(CRM_DASHBOARD_WIDGETS, (perms) => hasAnyPermission(...perms)),
    },
    { provide: DASHBOARD_LAYOUT_PERSIST, useFactory: () => hasAnyPermission(...CRM_MODULE_PERMISSIONS) },
  ];
}

/**
 * Bố cục Dashboard CRM đang hiện (PRD FR6/FR8/FR18, story-09). PROVIDE ở component Dashboard.
 *
 * Thứ tự dựng: (1) bản tạm localStorage ⇒ hiện khung NGAY (khối bắt đầu tải số luôn); (2) song song `GET layout` ⇒
 * gộp registry ⇒ khác bản đang hiện thì thay + ghi đè bản tạm; (3) API lỗi: giữ bản tạm, chưa có thì mặc định hệ thống.
 * Chưa có bản tạm ⇒ `layout()` = null tới khi API về (skeleton) — KHÔNG nháy mặc định hệ thống rồi đổi sang mặc định
 * thương hiệu (AC3).
 *
 * Chế độ sửa (story-10): `draft` là bản đang sửa, `layout` vẫn là bản ĐÃ LƯU (mốc của `dirty` và của Huỷ). Mọi lệnh ghi
 * (Lưu/Khôi phục/mặc định thương hiệu/thêm-ẩn ngoài chế độ sửa) huỷ `GET` ban đầu nếu nó chưa về — không thì GET cũ về sau
 * GHI ĐÈ bố cục vừa lưu (QA story-09). Mỗi lúc chỉ 1 lệnh ghi (`saving`), nên response không về lộn thứ tự.
 */
@Injectable()
export class DashboardLayoutStore {
  private readonly api = inject(CrmDashboardLayoutService);
  private readonly storageService = inject(StorageService);
  private readonly owner = inject(DASHBOARD_LAYOUT_OWNER);
  /** Khối user được thấy — MỌI phép dựng/gộp bố cục đi qua registry này (không dùng `CRM_DASHBOARD_WIDGETS` trực tiếp). */
  readonly registry = inject(DASHBOARD_WIDGET_REGISTRY);
  /** Được lưu bố cục (xem `DASHBOARD_LAYOUT_PERSIST`). Spec không provide ⇒ true. */
  readonly canPersist = inject(DASHBOARD_LAYOUT_PERSIST, { optional: true }) ?? true;
  /** Chữ ký từng mẫu bố cục (trên registry đã lọc quyền) ⇒ biết bản đang thấy trùng mẫu nào (đánh dấu trong menu). */
  private readonly presetSignatures = DASHBOARD_LAYOUT_PRESETS.map(
    (p) => [p.id, layoutSignature(presetLayout(p.id, this.registry))] as const,
  );
  private readonly storage: LayoutStorage = {
    load: () => this.storageService.loadStorage(STORAGE_KEY_SELECT.CrmDashboardLayout),
    save: (v) => this.storageService.saveStorage(STORAGE_KEY_SELECT.CrmDashboardLayout, v),
  };

  private readonly _layout = signal<DashboardLayoutConfig | null>(null);
  readonly layout = this._layout.asReadonly();
  readonly source = signal<CachedLayout['source']>('system');
  readonly hasBrandDefault = signal(false);
  /** Đã nhận bố cục từ server ít nhất 1 lần — `source` trước đó chỉ là bản tạm/mặc định, chưa đủ tin để hỏi chọn mẫu. */
  readonly serverLoaded = signal(false);

  readonly editing = signal(false);
  private readonly _draft = signal<DashboardLayoutConfig | null>(null);
  /** Một lệnh ghi đang chạy ⇒ khoá nút/kéo thả. */
  readonly saving = signal(false);
  /** Bố cục đang VẼ: bản nháp khi sửa, bản đã lưu khi xem. */
  readonly view = computed(() => (this.editing() ? this._draft() : this._layout()));
  readonly dirty = computed(() => {
    const d = this._draft();
    const l = this._layout();
    return this.editing() && !!d && !!l && layoutSignature(d) !== layoutSignature(l);
  });

  readonly top = computed(() => this.visible('top'));
  readonly main = computed(() => this.visible('main'));
  readonly side = computed(() => this.visible('side'));
  readonly visibleCount = computed(() => this.top().length + this.main().length + this.side().length);
  /** Mẫu bố cục trùng khớp bản đang thấy (null = bố cục tự sắp). */
  readonly activePreset = computed<DashboardPresetId | null>(() => {
    const v = this.view();
    if (!v) return null;
    const sig = layoutSignature(v);
    return this.presetSignatures.find(([, s]) => s === sig)?.[0] ?? null;
  });

  private initial: Subscription | null = null;

  constructor() {
    const cached = this.canPersist ? readCachedLayout(this.storage, this.owner) : null;
    if (cached) {
      this._layout.set(mergeLayout(cached.config, this.registry));
      this.source.set(cached.source);
    }
    if (!this.canPersist) {
      // Bố cục chỉ đọc: không gọi API (403), không dùng bản tạm (có thể là bản lúc còn quyền).
      this._layout.set(systemDefaultLayout(this.registry));
      return;
    }
    this.initial = this.api.getLayout().subscribe({
      next: (v) => this.applyServer(v, true),
      // Interceptor đã toast. Có bản tạm ⇒ giữ; chưa có ⇒ mặc định hệ thống để trang vẫn dùng được.
      error: () => {
        if (this._layout() === null) this._layout.set(systemDefaultLayout(this.registry));
      },
    });
    inject(DestroyRef).onDestroy(() => this.cancelInitial());
  }

  // ── Chế độ sửa ──

  startEdit(): void {
    const l = this._layout();
    if (!l || this.editing() || !this.canPersist) return;
    this._draft.set(l);
    this.editing.set(true);
  }

  cancelEdit(): void {
    this.editing.set(false);
    this._draft.set(null);
  }

  /** Áp 1 reducer (`layout.util.ts`) lên bản nháp. Ngoài chế độ sửa / đang lưu ⇒ bỏ qua. */
  edit(fn: (c: DashboardLayoutConfig) => DashboardLayoutConfig): void {
    const d = this._draft();
    if (!this.editing() || !d || this.saving()) return;
    this._draft.set(fn(d));
  }

  /**
   * Áp 1 mẫu bố cục (đủ mọi khối). Đang sửa ⇒ vào bản nháp (Huỷ để quay lại); đang xem ⇒ lưu luôn làm bố cục riêng
   * (lạc quan, lỗi thì trả lại) — cùng đường với Kho khối "+". Trả Observable (defer) ⇒ caller PHẢI subscribe.
   */
  applyPreset(id: DashboardPresetId): Observable<DashboardLayoutView> {
    return this.change(() => presetLayout(id, this.registry));
  }

  /** Lưu bản nháp làm bố cục của mình ⇒ thoát chế độ sửa. */
  save(): Observable<DashboardLayoutView> {
    const d = this._draft();
    if (!this.editing() || !d) return EMPTY;
    return this.write(() => this.api.saveLayout(d), () => this.cancelEdit());
  }

  /** Xoá bố cục riêng ⇒ về mặc định thương hiệu/hệ thống (BE trả bố cục hiệu lực). */
  reset(): Observable<DashboardLayoutView> {
    return this.write(() => this.api.resetLayout(), () => this.cancelEdit());
  }

  /**
   * Lưu bản đang thấy làm mặc định thương hiệu. Ai đã có bố cục riêng (kể cả chính admin) vẫn thấy bản riêng ⇒ admin có bản
   * riêng thì Ở LẠI chế độ sửa (bản nháp chưa thành bố cục của mình); không có ⇒ bố cục hiệu lực = bản vừa lưu ⇒ thoát.
   */
  saveBrandDefault(): Observable<DashboardLayoutView> {
    const cfg = this.editing() ? this._draft() : this._layout();
    if (!cfg) return EMPTY;
    return this.write(() => this.api.saveBrandDefault(cfg), (v) => {
      if (v.source !== 'user') this.cancelEdit();
    });
  }

  /**
   * Xoá mặc định thương hiệu (PRD FR7) ⇒ ai chưa có bố cục riêng về mặc định hệ thống. Đang có thay đổi chưa lưu ⇒ GIỮ nháp
   * (ở lại chế độ sửa, `dirty` so với bố cục hiệu lực mới); không ⇒ thoát sửa như Khôi phục mặc định.
   */
  deleteBrandDefault(): Observable<DashboardLayoutView> {
    return defer(() => {
      const keepDraft = this.dirty();
      return this.write(() => this.api.deleteBrandDefault(), () => {
        if (!keepDraft) this.cancelEdit();
      });
    });
  }

  /** Kho khối "+": đang sửa ⇒ vào bản nháp; đang xem ⇒ lưu luôn (story-10 AC4). */
  addWidget(key: string): Observable<DashboardLayoutView> {
    return this.change((c) => showItem(c, key, this.registry));
  }

  /** Ẩn khối (nút ẩn khi sửa / menu "…" của khối khi xem). */
  hideWidget(key: string): Observable<DashboardLayoutView> {
    return this.change((c) => hideItem(c, key));
  }

  // `defer`: chỉ đổi state khi được subscribe — gọi mà quên subscribe thì không khoá trang ở `saving`.
  private change(fn: (c: DashboardLayoutConfig) => DashboardLayoutConfig): Observable<DashboardLayoutView> {
    return defer(() => {
      if (this.editing()) {
        this.edit(fn);
        return EMPTY;
      }
      const prev = this._layout();
      if (!prev || this.saving() || !this.canPersist) return EMPTY;
      const next = fn(prev);
      if (next === prev) return EMPTY;
      this._layout.set(next); // lạc quan: hiện ngay, lỗi thì trả lại
      return this.write(() => this.api.saveLayout(next), undefined, () => {
        if (this._layout() === next) this._layout.set(prev);
      });
    });
  }

  private write(
    req: () => Observable<DashboardLayoutView>,
    onOk?: (v: DashboardLayoutView) => void,
    onErr?: () => void,
  ): Observable<DashboardLayoutView> {
    return defer(() => {
      if (this.saving() || !this.canPersist) return EMPTY;
      this.cancelInitial();
      this.saving.set(true);
      return this.run(req(), onOk, onErr);
    });
  }

  private run(
    req: Observable<DashboardLayoutView>,
    onOk?: (v: DashboardLayoutView) => void,
    onErr?: () => void,
  ): Observable<DashboardLayoutView> {
    return req.pipe(
      tap((v) => {
        this.applyServer(v);
        onOk?.(v);
      }),
      catchError((e: unknown) => {
        onErr?.();
        return throwError(() => e);
      }),
      finalize(() => this.saving.set(false)),
    );
  }

  private cancelInitial(): void {
    this.initial?.unsubscribe();
    this.initial = null;
  }

  /**
   * Áp bố cục server trả + ghi đè bản tạm. `fromInitial`: GET ban đầu về khi user ĐÃ vào chế độ sửa mà chưa đổi gì
   * (nháp = bản tạm) ⇒ nháp theo luôn bản server, không thì `dirty` bật oan.
   */
  applyServer(v: DashboardLayoutView, fromInitial = false): void {
    const merged = mergeLayout(v.source === 'system' ? null : v.config, this.registry);
    const cur = this._layout();
    if (fromInitial && this.editing() && cur && this._draft() && layoutSignature(this._draft()!) === layoutSignature(cur)) {
      this._draft.set(merged);
    }
    if (cur === null || layoutSignature(cur) !== layoutSignature(merged)) this._layout.set(merged);
    this.source.set(v.source);
    this.hasBrandDefault.set(!!v.hasBrandDefault);
    this.serverLoaded.set(true);
    writeCachedLayout(this.storage, this.owner, {
      config: v.source === 'system' ? null : merged,
      source: v.source,
      updatedAt: v.updatedAt ?? 0,
    });
  }

  private visible(zone: DashboardZone): DashboardLayoutItem[] {
    const l = this.view();
    return l ? l.items.filter((x) => x.zone === zone && x.visible) : [];
  }
}
