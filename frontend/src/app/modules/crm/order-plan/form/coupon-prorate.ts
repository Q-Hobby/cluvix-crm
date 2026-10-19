/**
 * Chia tiền coupon loại GIÁ TRỊ (`COUPON_TYPE.COUPON_TYPE_VALUE`) theo TỶ TRỌNG tiền từng dòng
 * dịch vụ của đơn — tách ra HÀM THUẦN để kiểm được bằng spec, không cần trình duyệt.
 *
 * ⚠ BÀI HỌC GỐC (bug tiền thật, /investigate 2026-09-21 — R-9): `_recalcMoney()` từng gán NGUYÊN
 * `coupon_value` cho MỖI dòng dịch vụ ⇒ đơn N dòng được giảm N× giá trị coupon, và cả
 * `order_plan.discount` lẫn `order_service_used.discount` đều mang số sai. Backend KHÔNG
 * chặn (`crm/service/order_plan.go` chỉ tự tính cho đơn 1 dòng và coupon PHẦN TRĂM) nên
 * số sai của FE được ghi thẳng vào DB.
 *
 * Căn cứ chọn cách chia: chính hệ thống đã prorate coupon giá trị tuyệt đối theo tỷ lệ doanh thu ở
 * `backend/internal/modules/webapp/crm/service/payment.go:913–920` — làm khác đi ở form đơn hàng là
 * mâu thuẫn với mô hình sẵn có.
 */

/**
 * @param bases  Tiền từng dòng SAU chiết khấu dịch vụ (`qty × unit_price − service_discount`),
 *               cùng thứ tự với mảng dòng dịch vụ. Giá trị âm/rỗng/không phải số ⇒ coi như 0.
 * @param couponValue Giá trị tuyệt đối của coupon (VND).
 * @returns Mảng tiền coupon của từng dòng, cùng độ dài `bases`. Bảo đảm:
 *  - `Σ kết quả === min(couponValue, Σ bases)` CHÍNH XÁC (phần dư do làm tròn dồn vào dòng CUỐI có
 *    `base > 0`; nếu dòng đó đã chạm trần thì tràn ngược lên các dòng có `base > 0` phía trước);
 *  - không phần tử nào vượt `base` của chính nó ⇒ không dòng nào ra tổng tiền âm;
 *  - dòng `base <= 0` luôn nhận 0 và KHÔNG nhận phần dư;
 *  - `Σ bases <= 0` hoặc `couponValue <= 0` ⇒ toàn 0 (không chia cho 0, không NaN).
 */
export function prorateValueCoupon(bases: number[], couponValue: number): number[] {
  const out: number[] = new Array(bases.length).fill(0);
  if (bases.length === 0) return out;

  const value = Math.round(Number(couponValue) || 0);
  if (value <= 0) return out;

  // Chuẩn hoá: chỉ dòng có tiền dương mới tham gia chia.
  const safeBases = bases.map((b) => {
    const n = Math.round(Number(b) || 0);
    return n > 0 ? n : 0;
  });
  const sumBases = safeBases.reduce((acc, b) => acc + b, 0);
  if (sumBases <= 0) return out;

  // Coupon lớn hơn (hoặc bằng) giá trị đơn ⇒ giảm tối đa bằng đúng tiền từng dòng, không âm.
  if (value >= sumBases) return safeBases.slice();

  let allocated = 0;
  for (let i = 0; i < safeBases.length; i++) {
    if (safeBases[i] <= 0) continue;
    const share = Math.min(Math.round((value * safeBases[i]) / sumBases), safeBases[i]);
    out[i] = share;
    allocated += share;
  }

  // Dồn phần dư (do làm tròn từng dòng) vào dòng CUỐI có `base > 0`; nếu dòng đó chạm trần `base`
  // (hoặc chạm sàn 0 khi dư âm) thì tràn ngược lên dòng có `base > 0` liền trước — nhờ vậy tổng
  // LUÔN đúng bằng `couponValue`, không lệch 1đ.
  let residual = value - allocated;
  for (let i = safeBases.length - 1; i >= 0 && residual !== 0; i--) {
    if (safeBases[i] <= 0) continue;
    const room = residual > 0 ? safeBases[i] - out[i] : -out[i];
    const delta = residual > 0 ? Math.min(residual, room) : Math.max(residual, room);
    out[i] += delta;
    residual -= delta;
  }

  return out;
}
