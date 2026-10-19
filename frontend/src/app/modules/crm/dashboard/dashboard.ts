import { BreakpointObserver } from '@angular/cdk/layout';
import { CdkDrag, CdkDragDrop, CdkDropList, CdkDropListGroup } from '@angular/cdk/drag-drop';
import {
  afterNextRender, ChangeDetectionStrategy, Component, computed, DestroyRef, ElementRef, HostListener, inject, Injector, signal,
} from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { TranslatePipe, TranslateService } from '@ngx-translate/core';
import { MenuItem } from 'primeng/api';
import type { ButtonSeverity } from 'primeng/button';
import { ButtonModule } from 'primeng/button';
import { MenuModule } from 'primeng/menu';
import { TooltipModule } from 'primeng/tooltip';
import { Skeleton } from 'primeng/skeleton';
import { firstValueFrom, map } from 'rxjs';

import { hasPermission } from '@/core/services/auth/auth.selectors';
import { PERMISSION_SELECT, STORAGE_KEY_SELECT } from '@/define';
import { AppConfirmDialog } from '@/share/components/app-confirm-dialog';
import { DialogWrapperService } from '@/share/services/dialog-wrapper.service';
import { NotificationService } from '@/share/services/notification.service';
import { StorageService } from '@/share/services/storage.service';

import { DashboardFilterBar } from './shell/dashboard-filter-bar';
import { DashboardFilterStore } from './shell/dashboard-filter.store';
import { DashboardLayoutStore, provideDashboardLayoutOwner, provideDashboardWidgetRegistry } from './shell/dashboard-layout.store';
import type { DashboardZone } from './shell/dashboard-widgets';
import { DASHBOARD_ZONES, type DashboardLayoutItem, moveBy, placeItem, setItemSize } from './shell/layout.util';
import type { WidgetMoveCommand } from './shell/widget-edit-chrome';
import { DashboardWidgetGallery } from './shell/widget-gallery';
import { DASHBOARD_LAYOUT_PRESETS, type DashboardPresetId } from './shell/layout-presets';
import { DashboardPresetChooser } from './shell/preset-chooser';
import { DashboardWidgetHost } from './shell/widget-host';
import { WIDGET_VIEW_POOL_COMPONENT, WidgetViewPool } from './shell/widget-view-pool';
import { DashboardZoneItem } from './shell/zone-item';

const FLASH_MS = 1600;

/**
 * Dashboard CRM tuỳ biến (port từ ERP epic crm-dashboard-customizable): thanh lọc chung + shell 3 vùng dựng từ bố cục +
 * chế độ "Tuỳ chỉnh" kéo thả / ẩn / đổi cỡ / Kho khối / lưu – khôi phục – mặc định thương hiệu.
 * CRM thêm gác quyền khối: chỉ khối user được thấy (`DashboardLayoutStore.registry`) mới vào bố cục/Kho khối/mẫu.
 *
 * KHÔNG import tĩnh các khối — mỗi khối tải chunk riêng khi bật & tới gần. View khối nằm trong `WidgetViewPool` ⇒ kéo
 * sang vùng khác KHÔNG mount lại, không gọi lại API.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './dashboard.html',
  styleUrls: ['./dashboard.scss'],
  imports: [
    DashboardFilterBar, DashboardWidgetGallery, DashboardZoneItem, DashboardPresetChooser, Skeleton, ButtonModule, MenuModule, TooltipModule, TranslatePipe,
    CdkDropListGroup, CdkDropList, CdkDrag,
  ],
  providers: [
    DashboardFilterStore,
    DashboardLayoutStore,
    provideDashboardLayoutOwner(),
    provideDashboardWidgetRegistry(),
    WidgetViewPool,
    { provide: WIDGET_VIEW_POOL_COMPONENT, useValue: DashboardWidgetHost },
  ],
})
export class Dashboard {
  protected readonly layout = inject(DashboardLayoutStore);
  private readonly dialog = inject(DialogWrapperService);
  private readonly notify = inject(NotificationService);
  private readonly translate = inject(TranslateService);
  private readonly storage = inject(StorageService);
  private readonly injector = inject(Injector);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly destroyRef = inject(DestroyRef);

  /** Kéo thả chỉ ở ≥ md (PRD FR26) — nhỏ hơn thì dùng menu di chuyển. */
  protected readonly desktop = toSignal(
    inject(BreakpointObserver).observe('(min-width: 768px)').pipe(map((s) => s.matches)),
    { initialValue: false },
  );
  /** `computed` ⇒ quyền nạp muộn (authInitialized) vẫn hiện nút. */
  protected readonly canManageConfig = computed(() => hasPermission(PERMISSION_SELECT.permissionManageConfig));

  protected readonly galleryOpen = signal(false);
  protected readonly flashKey = signal<string | null>(null);
  private flashTimer: ReturnType<typeof setTimeout> | undefined;

  private readonly hintSeen = signal(this.readHintSeen());
  private readonly chooserSeen = signal(!!this.storage.loadStorage(STORAGE_KEY_SELECT.CrmDashboardPresetChooserSeen));
  /** "Chọn bố cục khởi đầu": chỉ khi server XÁC NHẬN đang dùng bố cục hệ thống (chưa bản riêng, chưa mặc định thương hiệu). */
  protected readonly showChooser = computed(
    () => this.layout.serverLoaded() && this.layout.source() === 'system' && !this.layout.editing() && !this.chooserSeen(),
  );
  protected readonly showHint = computed(() => !this.hintSeen() && !!this.layout.layout() && !this.layout.editing() && !this.showChooser());
  protected readonly empty = computed(() => !!this.layout.view() && this.layout.visibleCount() === 0);
  protected readonly dragOff = computed(() => !this.layout.editing() || !this.desktop() || this.layout.saving());

  protected itemClass(it: DashboardLayoutItem, zone: DashboardZone): string {
    const span = zone === 'main' && it.size === 'full' ? ' md:col-span-2' : '';
    return `dash-item min-w-0${span}${this.flashKey() === it.key ? ' dash-flash' : ''}`;
  }

  constructor() {
    this.destroyRef.onDestroy(() => clearTimeout(this.flashTimer));
  }

  // ── Chế độ sửa ──

  protected startEdit(): void {
    this.dismissHint();
    this.layout.startEdit();
  }

  protected cancelEdit(): void {
    this.layout.cancelEdit();
  }

  /** Menu "Mẫu bố cục" — nhãn MenuItem là chuỗi ⇒ `stream` (đổi theo ngôn ngữ/bản dịch nạp muộn), dấu ✓ ở mẫu đang khớp. */
  private readonly presetLabels = toSignal(this.translate.stream(DASHBOARD_LAYOUT_PRESETS.map((p) => p.labelKey)), {
    initialValue: {} as Record<string, string>,
  });
  protected readonly presetItems = computed<MenuItem[]>(() => {
    const active = this.layout.activePreset();
    return DASHBOARD_LAYOUT_PRESETS.map((p) => ({
      label: this.presetLabels()[p.labelKey],
      icon: p.id === active ? 'pi pi-check' : p.icon,
      styleClass: p.id === active ? 'font-semibold' : undefined,
      command: () => void this.pickPreset(p.id),
    }));
  });

  protected save(): void {
    this.layout.save().subscribe({ next: () => this.notify.successMsg(this.translate.instant('common.label.saved')), error: () => undefined });
  }

  protected async reset(): Promise<void> {
    const ok = await this.ask('dashboard.crm.layout.reset_confirm_title', 'dashboard.crm.layout.reset_confirm_message', 'dashboard.crm.layout.reset');
    if (!ok) return;
    this.layout.reset().subscribe({ next: () => this.notify.successMsg(this.translate.instant('dashboard.crm.layout.reset_done')), error: () => undefined });
  }

  protected async saveBrandDefault(): Promise<void> {
    const ok = await this.ask(
      'dashboard.crm.layout.brand_confirm_title', 'dashboard.crm.layout.brand_confirm_message', 'dashboard.crm.layout.save_brand_default',
      'primary',
    );
    if (!ok) return;
    this.layout.saveBrandDefault().subscribe({
      next: () => this.notify.successMsg(this.translate.instant('dashboard.crm.layout.saved_brand')),
      error: () => undefined,
    });
  }

  protected async deleteBrandDefault(): Promise<void> {
    const ok = await this.ask(
      'dashboard.crm.layout.delete_brand_confirm_title', 'dashboard.crm.layout.delete_brand_confirm_message',
      'dashboard.crm.layout.delete_brand_default',
    );
    if (!ok) return;
    this.layout.deleteBrandDefault().subscribe({
      next: () => this.notify.successMsg(this.translate.instant('dashboard.crm.layout.deleted_brand')),
      error: () => undefined,
    });
  }

  // `[cdkDropListData]="'main'"` được suy ra kiểu `string` ⇒ nhận `string` rồi thu hẹp.
  protected onDrop(e: CdkDragDrop<string, string, string>): void {
    const zone = DASHBOARD_ZONES.find((z) => z === e.container.data);
    if (!zone || (e.previousContainer === e.container && e.previousIndex === e.currentIndex)) return;
    this.layout.edit((c) => placeItem(c, e.item.data, zone, e.currentIndex));
  }

  protected onMove(it: DashboardLayoutItem, cmd: WidgetMoveCommand): void {
    this.layout.edit((c) =>
      cmd.kind === 'zone' ? placeItem(c, it.key, cmd.zone, Number.MAX_SAFE_INTEGER) : moveBy(c, it.key, cmd.kind === 'up' ? -1 : 1),
    );
    // Đổi vùng ⇒ chrome + nút Di chuyển dựng lại; cùng vùng ⇒ DOM bị dời ⇒ focus rơi về <body>. Trả focus về nút (QA P4).
    this.reveal(it.key, false, true);
  }

  protected toggleSize(it: DashboardLayoutItem): void {
    this.layout.edit((c) => setItemSize(c, it.key, it.size === 'full' ? 'half' : 'full'));
  }

  protected hide(key: string): void {
    this.layout.hideWidget(key).subscribe({ error: () => undefined });
  }

  // ── Kho khối ──

  protected openGallery(): void {
    this.dismissHint();
    this.galleryOpen.set(true);
  }

  protected add(key: string): void {
    this.galleryOpen.set(false);
    this.layout.addWidget(key).subscribe({ error: () => undefined });
    this.reveal(key, true);
  }

  /** Cuộn tới khối (+ nhấp nháy viền khi `flash`, + focus nút Di chuyển khi `focusMove`) sau khi nó được vẽ ở chỗ mới. */
  private reveal(key: string, flash: boolean, focusMove = false): void {
    if (flash) {
      clearTimeout(this.flashTimer);
      this.flashKey.set(key);
      this.flashTimer = setTimeout(() => this.flashKey.update((k) => (k === key ? null : k)), FLASH_MS);
    }
    afterNextRender(
      {
        read: () => {
          const el = this.host.nativeElement.querySelector<HTMLElement>(`[data-widget-key="${key}"]`);
          const reduce = typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches;
          el?.scrollIntoView({ behavior: reduce ? 'auto' : 'smooth', block: 'nearest' });
          if (focusMove) el?.querySelector<HTMLElement>('[data-move-btn]')?.focus({ preventScroll: true });
        },
      },
      { injector: this.injector },
    );
  }

  // ── Gợi ý lần đầu (cờ theo THIẾT BỊ, không xoá khi đăng xuất) ──

  /**
   * Chọn mẫu bố cục. Đang sửa ⇒ vào bản nháp. Đang xem ⇒ lưu luôn thành bố cục riêng; đã có bố cục tự sắp ⇒ hỏi trước
   * (bị thay hẳn, "Khôi phục mặc định" không đem bản cũ về).
   */
  protected async pickPreset(id: DashboardPresetId): Promise<void> {
    if (this.layout.editing()) {
      this.layout.applyPreset(id).subscribe({ error: () => undefined });
      return;
    }
    if (this.layout.activePreset() === id) return;
    if (this.layout.source() === 'user') {
      const ok = await this.ask(
        'dashboard.crm.layout.preset_confirm_title', 'dashboard.crm.layout.preset_confirm_message', 'dashboard.crm.layout.preset_confirm_ok', 'primary', 'pi pi-th-large',
      );
      if (!ok) return;
    }
    this.layout.applyPreset(id).subscribe({
      next: () => {
        this.dismissChooser();
        this.notify.successMsg(this.translate.instant('dashboard.crm.layout.preset_applied'));
      },
      error: () => undefined, // interceptor đã toast, store đã trả lại bố cục cũ
    });
  }

  protected dismissChooser(): void {
    if (this.chooserSeen()) return;
    this.chooserSeen.set(true);
    this.storage.saveStorage(STORAGE_KEY_SELECT.CrmDashboardPresetChooserSeen, 1);
  }

  protected dismissHint(): void {
    if (this.hintSeen()) return;
    this.hintSeen.set(true);
    this.storage.saveStorage(STORAGE_KEY_SELECT.CrmDashboardHintSeen, 1);
  }

  private readHintSeen(): boolean {
    return !!this.storage.loadStorage(STORAGE_KEY_SELECT.CrmDashboardHintSeen);
  }

  // ── Rời trang khi còn thay đổi chưa lưu (khuôn `menu-layout-config.ts`) ──

  @HostListener('window:beforeunload', ['$event'])
  onBeforeUnload(event: BeforeUnloadEvent): void {
    if (!this.layout.dirty()) return;
    event.preventDefault();
    // Trình duyệt cũ (Chrome < 119) chỉ hỏi khi `returnValue` được gán.
    event.returnValue = '';
  }

  /** `canDeactivate` của route: có thay đổi chưa lưu ⇒ hỏi bỏ thay đổi. */
  confirmDiscard(): Promise<boolean> {
    if (!this.layout.dirty()) return Promise.resolve(true);
    return this.ask('dashboard.crm.layout.unsaved_title', 'dashboard.crm.layout.unsaved_message', 'dashboard.crm.layout.discard');
  }

  /** Hộp xác nhận chuẩn `AppConfirmDialog` (khuôn `BaseSignalForm.closeForm`). Đóng bằng X/Esc/mask ⇒ huỷ. */
  private ask(
    titleKey: string, messageKey: string, acceptLabel: string, acceptSeverity: ButtonSeverity = 'danger', icon?: string,
  ): Promise<boolean> {
    return firstValueFrom(
      this.dialog
        .open(AppConfirmDialog, {
          data: {
            title: this.translate.instant(titleKey),
            message: this.translate.instant(messageKey),
            icon: icon ?? (acceptSeverity === 'danger' ? 'pi pi-exclamation-triangle' : 'pi pi-building'),
            acceptSeverity,
            acceptLabel,
            rejectLabel: 'common.action.cancel',
          },
          showHeader: false,
          styleClass: 'shadow-sm rounded-3xl',
          contentStyle: { padding: 0 },
          width: '40rem',
        })
        .onClose.pipe(map((r) => r === true)),
    );
  }
}
