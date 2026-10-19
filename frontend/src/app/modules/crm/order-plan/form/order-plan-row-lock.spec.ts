import { describe, expect, it } from 'vitest';

import { isRowFieldLocked } from './order-plan-row-lock';

/**
 * Khoá ô dòng dịch vụ ở luồng đơn hàng thường — 2 khoá FE-only duy nhất còn lại sau khi bỏ chế độ
 * "chốt dự toán" (tính năng đã xoá cùng BE `treatment_estimate`, story-53 D19b).
 */
describe('isRowFieldLocked — luồng đơn hàng thường', () => {
  it('không khoá ô nào khi dòng không có cờ nào', () => {
    expect(isRowFieldLocked('unit_price', {})).toBe(false);
    expect(isRowFieldLocked('service_id', {})).toBe(false);
    expect(isRowFieldLocked('unit_price', null)).toBe(false);
    expect(isRowFieldLocked('service_id', null)).toBe(false);
  });

  it('unit_price theo unitPriceLocked, service_id theo serviceLocked — không lan sang ô khác', () => {
    expect(isRowFieldLocked('unit_price', { unitPriceLocked: true })).toBe(true);
    expect(isRowFieldLocked('service_id', { serviceLocked: true })).toBe(true);
    expect(isRowFieldLocked('service_id', { unitPriceLocked: true })).toBe(false);
    expect(isRowFieldLocked('unit_price', { serviceLocked: true })).toBe(false);
  });
});
