import { describe, expect, it, vi } from 'vitest';

import { FormMoveDeposit } from './form-move-deposit';

/**
 * Neo lens-money M-4: số tiền chuyển cọc ≤ min(số dư cọc, nợ còn lại của dịch vụ đã chọn).
 * Gọi hàm sản phẩm với `this` giả; kỳ vọng bằng số cụ thể.
 */
type Fake = { max: number; serviceUsed: () => { value: number; number: number }[]; rootField?: unknown; translate?: unknown };
const proto = FormMoveDeposit.prototype as unknown as {
  amountLimit(this: Fake, id: number | null): number | null;
  maxDeposit(this: Fake): void;
};

const services = [
  { value: 1, number: 800_000 },
  { value: 2, number: 5_000_000 },
];

describe('FormMoveDeposit.amountLimit (M-4)', () => {
  it('cọc lớn hơn nợ dịch vụ ⇒ trần = nợ dịch vụ', () => {
    expect(proto.amountLimit.call({ max: 2_000_000, serviceUsed: () => services }, 1)).toBe(800_000);
  });

  it('nợ dịch vụ lớn hơn cọc ⇒ trần = số dư cọc', () => {
    expect(proto.amountLimit.call({ max: 2_000_000, serviceUsed: () => services }, 2)).toBe(2_000_000);
  });

  it('chế độ Sửa (không có danh sách nợ) ⇒ chỉ giới hạn theo cọc', () => {
    expect(proto.amountLimit.call({ max: 2_000_000, serviceUsed: () => [] }, 1)).toBe(2_000_000);
  });

  it('không có cọc lẫn dịch vụ ⇒ không giới hạn (null)', () => {
    expect(proto.amountLimit.call({ max: 0, serviceUsed: () => [] }, null)).toBeNull();
  });
});

describe('FormMoveDeposit.maxDeposit', () => {
  const run = (max: number, serviceId: number) => {
    const set = vi.fn();
    const fake = {
      max,
      serviceUsed: () => services,
      rootField: { serviceUsed_id: () => ({ value: () => serviceId }), amount: () => ({ value: { set } }) },
      translate: { instant: (k: string) => k },
    };
    vi.spyOn(console, 'warn').mockImplementation(() => undefined);
    proto.maxDeposit.call(fake);
    return set;
  };

  it('điền min(số dư cọc, nợ dịch vụ)', () => {
    expect(run(2_000_000, 1)).toHaveBeenCalledWith(800_000);
    expect(run(2_000_000, 2)).toHaveBeenCalledWith(2_000_000);
  });

  it('dịch vụ không tồn tại ⇒ không điền', () => {
    expect(run(2_000_000, 99)).not.toHaveBeenCalled();
  });
});
