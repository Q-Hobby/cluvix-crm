import { DestroyRef, Directive, effect, inject, model, output, signal, untracked } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { EMPTY, Observable, catchError, finalize } from 'rxjs';

import { BaseApp } from '@/share/base/BaseApp';
import { detailLoadErrorKey } from '@/share/components/slide-over';

/**
 * Khung chung cho các panel "chi tiết bản ghi" dạng SlideOver (chi tiết lịch hẹn, đơn hàng, thanh
 * toán, điều trị, dịch vụ điều trị...).
 *
 * Base này KHÔNG ôm bố cục field — mỗi nghiệp vụ hiển thị thứ khác nhau, đó là lý do panel tồn tại.
 * Nó chỉ ôm **máy móc nạp dữ liệu** vốn giống hệt nhau ở mọi panel và trước đây bị chép tay 5 lần:
 *
 * - vẽ ngay bằng `seed` (dòng list đã `addLabel`) rồi mới gọi API → không có khoảng trắng chờ;
 * - `loadSeq` chống race: bấm nhanh 2 bản ghi thì response về trễ của lần trước KHÔNG được đè lên
 *   bản ghi đang xem (bug này im lặng và rất khó thấy khi test tay);
 * - lỗi nạp báo NGAY TRONG panel (`loadErrorKey`) + "Thử lại", trang nền giữ nguyên — đi cùng
 *   `SKIP_GLOBAL_404` bên `response-handler.interceptor`;
 * - dọn `model` khi đóng để lần mở sau không loé dữ liệu bản ghi cũ.
 *
 * Vì sao dùng hook chứ không ép một khuôn cứng: các panel khác nhau thật ở 3 điểm — tên input id
 * (giữ nguyên để 21 chỗ gọi không phải sửa), cách nạp cache master-data, và cách gộp seed với
 * response (có panel cố ý KHÔNG gộp seed). Ba điểm đó là `currentId`/`fetchDetail`/`decorate`.
 *
 * @example
 * ```ts
 * export class PaymentDetailPanel extends BaseDetailPanel<PaymentModel> {
 *   readonly paymentId = input<number | null>(null);
 *   readonly seed = input<PaymentModel | null>(null);
 *
 *   protected currentId() { return this.paymentId(); }
 *   protected currentSeed() { return this.seed(); }
 *   protected fetchDetail(id: number) { return this.paymentService.getDetail(id); }
 *   protected override ensureCache() { return this.paymentService.ensureCacheLoaded(); }
 *   protected override decorate(detail: PaymentModel, seed: PaymentModel | null) {
 *     return this.paymentService.addLabel({ ...(seed ?? {}), ...detail }) as PaymentModel;
 *   }
 * }
 * ```
 */
@Directive()
export abstract class BaseDetailPanel<T extends { id?: number | string }> extends BaseApp {
  /** Đóng/mở panel — two-way với trang gọi: `[(visible)]="detailVisible"`. */
  readonly visible = model(false);
  /** Bắn ra SAU khi panel đã đóng hẳn (SlideOver `(closed)`). */
  readonly closed = output<void>();

  readonly model = signal<T>({} as T);
  readonly loadingDetail = signal(false);
  /** Khoá i18n của lỗi nạp chi tiết ('' = không lỗi) — panel tự báo, trang nền giữ nguyên. */
  readonly loadErrorKey = signal('');

  protected readonly destroyRef = inject(DestroyRef);

  /** Bộ đếm request — response về trễ của lần mở trước bị bỏ (chống race khi bấm nhanh 2 bản ghi). */
  private loadSeq = 0;

  constructor() {
    super();

    effect(() => {
      const open = this.visible();
      const id = this.currentId();
      if (!open || !id) return;
      // untracked: loadDetail đọc/ghi nhiều signal khác, không được biến chúng thành phụ thuộc
      // của effect này (sẽ tự kích lại chính nó).
      untracked(() => void this.loadDetail(Number(id)));
    });
  }

  // ── Hook bắt buộc ────────────────────────────────────────────────────────────

  /** Id bản ghi đang mở. Trả về input riêng của panel (`paymentId`, `appointmentId`...). */
  protected abstract currentId(): number | null;
  /** Row sẵn có từ list để vẽ ngay; `null` nếu trang gọi không truyền `[seed]`. */
  protected abstract currentSeed(): T | null;
  /** Gọi API chi tiết. */
  protected abstract fetchDetail(id: number): Observable<T>;

  // ── Hook tuỳ chọn ────────────────────────────────────────────────────────────

  /** Nạp cache master-data (user, chi nhánh, dịch vụ...) trước khi gắn nhãn. */
  protected ensureCache(): Promise<unknown> {
    return Promise.resolve();
  }

  /**
   * Biến response thành model để hiển thị. Mặc định: gộp seed rồi để panel gắn nhãn ở lớp con.
   * `seed` là `null` khi trang gọi không truyền hoặc seed không phải bản ghi này.
   */
  protected decorate(detail: T, seed: T | null): T {
    return { ...(seed ?? ({} as T)), ...detail };
  }

  /** Chạy đầu mỗi lượt nạp — dùng để reset state phụ của panel (ô nhập ghi chú...). */
  protected onLoadStart(): void {
    // Override khi cần
  }

  // ── Máy móc ──────────────────────────────────────────────────────────────────

  protected async loadDetail(id: number): Promise<void> {
    const seed = this.currentSeed();
    // So sánh lỏng: id từ input là number, `seed.id` có thể là chuỗi (API/list cũ).
    const usableSeed = seed && Number(seed.id) === id ? seed : null;
    const seq = ++this.loadSeq;

    this.onLoadStart();
    this.model.set(usableSeed ?? ({} as T));
    // Có seed thì vẽ được ngay → không hiện spinner, tránh nháy khung.
    this.loadingDetail.set(!usableSeed);
    this.loadErrorKey.set('');

    await this.ensureCache();
    // Mở nhanh 2 bản ghi liên tiếp: lần mở sau đã tăng loadSeq → lần trước dừng ngay tại đây.
    if (seq !== this.loadSeq) return;

    this.fetchDetail(id)
      .pipe(
        takeUntilDestroyed(this.destroyRef),
        catchError((err: unknown) => {
          // Lần mở sau đã ghi đè → lỗi của lần trước không được phép hiện lên panel đang xem.
          if (seq === this.loadSeq) this.loadErrorKey.set(detailLoadErrorKey(err));
          return EMPTY;
        }),
        finalize(() => {
          if (seq === this.loadSeq) this.loadingDetail.set(false);
        }),
      )
      .subscribe((detail) => {
        if (seq !== this.loadSeq) return;
        this.model.set(this.decorate(detail, usableSeed));
      });
  }

  /** Bấm "Thử lại" ở trạng thái lỗi — nạp lại đúng bản ghi đang mở. */
  protected retryLoad(): void {
    const id = this.model()?.id ?? this.currentId();
    if (id) void this.loadDetail(Number(id));
  }

  /** SlideOver `(closed)`. Lớp con override thì PHẢI gọi `super.onClosed()`. */
  protected onClosed(): void {
    this.model.set({} as T);
    this.closed.emit();
  }
}
