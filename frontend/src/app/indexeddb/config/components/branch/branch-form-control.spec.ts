import { SimpleChange, SimpleChanges } from '@angular/core';
import { describe, expect, it, vi } from 'vitest';

import { BranchFormControl } from './branch-form-control';

/**
 * Các input lọc của `BranchFormControl` giữ nguyên hành vi cũ: đổi là reset thẳng.
 * (Story-61: `companyId` đã bỏ khỏi component — spec riêng `company-filter-reset.spec.ts` đã xoá.)
 *
 * Gọi thẳng `prototype.ngOnChanges` với host giả thay vì `TestBed.createComponent`: thân hàm chỉ đụng
 * `this.selectedItem` + `this.onChange` (không DOM, không service), nên không cần dựng cả cây DI
 * (TranslateService, BranchCacheService → IndexedDB). Vẫn là lưới THẬT: import file sản phẩm và chạy
 * đúng hàm của nó, không chép lại điều kiện.
 */
const ngOnChanges = BranchFormControl.prototype.ngOnChanges as (this: unknown, c: SimpleChanges) => void;

function makeHost() {
  const onChange = vi.fn();
  return {
    onChange,
    host: { selectedItem: { id: 7, name: 'Chi nhánh cũ' }, onChange },
  };
}

describe('BranchFormControl.ngOnChanges — input lọc', () => {
  it('activeOnly đổi → vẫn reset như cũ', () => {
    const { host, onChange } = makeHost();

    ngOnChanges.call(host, { activeOnly: new SimpleChange(false, true, false) });

    expect(onChange).toHaveBeenCalledWith(null);
    expect(host.selectedItem).toBeNull();
  });

  it('statusOpenAppointment đổi → vẫn reset như cũ', () => {
    const { host, onChange } = makeHost();

    ngOnChanges.call(host, { statusOpenAppointment: new SimpleChange(false, true, false) });

    expect(onChange).toHaveBeenCalledWith(null);
    expect(host.selectedItem).toBeNull();
  });
});
