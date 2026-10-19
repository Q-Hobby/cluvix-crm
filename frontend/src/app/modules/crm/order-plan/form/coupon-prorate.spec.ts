import { describe, expect, it } from 'vitest';

import { prorateValueCoupon } from './coupon-prorate';

/**
 * Lưới khoá công thức chia coupon GIÁ TRỊ theo dòng (story-01, bug tiền thật R-9).
 *
 * Chạy: `npx vitest run -c vitest-base.config.ts src/app/modules/crm/order-plan/form/coupon-prorate.spec.ts`
 * (thiếu `-c` ⇒ mất `setupFiles` ⇒ đỏ giả).
 *
 * ⚠ Mọi kỳ vọng dưới đây viết bằng SỐ CỤ THỂ, KHÔNG gọi lại chính hàm sản phẩm để so — tự chép
 * công thức vào spec thì spec luôn xanh kể cả khi code sản phẩm hỏng.
 */
const sum = (arr: number[]) => arr.reduce((a, b) => a + b, 0);

describe('prorateValueCoupon', () => {
  // AC1 — chia theo tỷ trọng, tổng đúng bằng giá trị coupon
  it('chia theo tỷ trọng tiền dòng và tổng bằng đúng coupon_value', () => {
    const result = prorateValueCoupon([1_000_000, 500_000, 500_000], 200_000);
    expect(result).toEqual([100_000, 50_000, 50_000]);
    expect(sum(result)).toBe(200_000);
  });

  // AC1 — ca lẻ dư: 100.000 chia 3 phần bằng nhau ⇒ 33.333 / 33.333 / 33.334
  it('dồn phần dư làm tròn vào dòng CUỐI có base > 0', () => {
    const result = prorateValueCoupon([300_000, 300_000, 300_000], 100_000);
    expect(result).toEqual([33_333, 33_333, 33_334]);
    expect(sum(result)).toBe(100_000);
  });

  it('dòng cuối base = 0 thì phần dư dồn vào dòng có base > 0 cuối cùng', () => {
    const result = prorateValueCoupon([300_000, 300_000, 300_000, 0], 100_000);
    expect(result).toEqual([33_333, 33_333, 33_334, 0]);
    expect(sum(result)).toBe(100_000);
  });

  // AC1 — dư ÂM (tổng các phần làm tròn lên vượt coupon_value) vẫn phải khớp tuyệt đối
  it('tổng vẫn đúng khi làm tròn lên gây dư âm', () => {
    const result = prorateValueCoupon([2, 2, 2, 2, 2, 2], 3);
    expect(sum(result)).toBe(3);
    result.forEach((d, i) => expect(d).toBeLessThanOrEqual([2, 2, 2, 2, 2, 2][i]));
  });

  // AC2 — coupon ≥ giá trị đơn ⇒ cap theo từng dòng, không âm
  it('coupon lớn hơn giá trị đơn thì mỗi dòng chỉ giảm tối đa bằng tiền dòng đó', () => {
    const bases = [100_000, 50_000];
    const result = prorateValueCoupon(bases, 500_000);
    expect(result).toEqual([100_000, 50_000]);
    expect(sum(result)).toBe(150_000);
    result.forEach((d, i) => expect(bases[i] - d).toBeGreaterThanOrEqual(0));
  });

  it('coupon đúng bằng giá trị đơn ⇒ mỗi dòng về 0đ, không dòng nào âm', () => {
    const result = prorateValueCoupon([100_000, 50_000], 150_000);
    expect(result).toEqual([100_000, 50_000]);
  });

  // AC3 — dòng base = 0 và ca mọi dòng = 0
  it('dòng base = 0 nhận 0 và không nhận phần dư', () => {
    const result = prorateValueCoupon([0, 1_000_000, 0], 250_000);
    expect(result).toEqual([0, 250_000, 0]);
  });

  it('mọi dòng base = 0 ⇒ toàn 0, không NaN', () => {
    const result = prorateValueCoupon([0, 0, 0], 200_000);
    expect(result).toEqual([0, 0, 0]);
    result.forEach((d) => expect(Number.isNaN(d)).toBe(false));
  });

  it('base âm/không hợp lệ được coi như 0', () => {
    const result = prorateValueCoupon([-50_000, 1_000_000], 100_000);
    expect(result).toEqual([0, 100_000]);
  });

  // AC4 — đơn 1 dòng giữ nguyên hành vi cũ (giảm đúng giá trị coupon)
  it('đơn 1 dòng ⇒ giảm đúng coupon_value như trước', () => {
    expect(prorateValueCoupon([1_000_000], 200_000)).toEqual([200_000]);
  });

  // Biên: coupon 0/âm, mảng rỗng
  it('coupon_value <= 0 hoặc không có dòng nào ⇒ toàn 0', () => {
    expect(prorateValueCoupon([1_000_000, 500_000], 0)).toEqual([0, 0]);
    expect(prorateValueCoupon([1_000_000], -5)).toEqual([0]);
    expect(prorateValueCoupon([], 100_000)).toEqual([]);
  });
});
