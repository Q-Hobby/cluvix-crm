import { signal } from '@angular/core';

/**
 * State của một panel chi tiết (SlideOver) nhìn từ phía TRANG GỌI.
 *
 * Trước đây mỗi list tự khai 3 signal `detailVisible/detailId/detailSeed` + một hàm `open...`
 * y hệt nhau — 21 chỗ, 21 bản chép. Gom lại đây để:
 * - thêm/đổi hành vi mở panel (vd nhớ bản ghi vừa xem, mở theo id từ URL) chỉ sửa 1 nơi;
 * - không còn kiểu "quên `if (!model?.id) return;`" ở một list nào đó — bug im lặng vì bấm vào
 *   dòng thiếu id sẽ mở panel rỗng thay vì không làm gì.
 *
 * KHÔNG phải service: mỗi list cần state RIÊNG, và panel có thể mở nhiều nơi cùng lúc trên cùng
 * một màn (tab hồ sơ khách hàng). Đây chỉ là một object state nhỏ, tạo bằng field initializer.
 *
 * @example
 * ```ts
 * protected readonly detail = detailPanelHost<PaymentModel>();
 * openPaymentDetail(model: PaymentModel) { this.detail.open(model); }
 * ```
 * ```html
 * @defer (when detail.visible(); prefetch on idle) {
 *   <app-payment-detail-panel
 *     [(visible)]="detail.visible"
 *     [paymentId]="detail.id()"
 *     [seed]="detail.seed()"
 *   />
 * }
 * ```
 */
export class DetailPanelHost<T extends { id?: number | string }> {
  readonly visible = signal(false);
  readonly id = signal<number | null>(null);
  /** Row của list (đã `addLabel`) để panel vẽ ngay trong lúc chờ API chi tiết. */
  readonly seed = signal<T | null>(null);

  /** Mở panel cho một dòng. Dòng không có id thì KHÔNG mở (panel rỗng vô nghĩa). */
  open(model: T | null | undefined): void {
    if (!model?.id) return;
    this.seed.set(model);
    this.id.set(Number(model.id));
    this.visible.set(true);
  }

  close(): void {
    this.visible.set(false);
  }
}

export function detailPanelHost<T extends { id?: number | string }>(): DetailPanelHost<T> {
  return new DetailPanelHost<T>();
}
