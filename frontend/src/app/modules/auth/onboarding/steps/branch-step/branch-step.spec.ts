import { TestBed } from '@angular/core/testing';
import { form } from '@angular/forms/signals';
import { signal } from '@angular/core';
import { describe, expect, it } from 'vitest';

import {
  BranchStepModel,
  buildBranchStepSchema,
  buildBranchStepValue,
  computeBranchStepIsEmpty,
  emptyBranchStep,
} from './branch-step';

/**
 * Story-06 — lưới tối thiểu cho logic THUẦN của bước "Chi nhánh": `isEmpty()`/`value()`/`valid()`
 * (theo contract `OnboardingStepComponent`), cùng khuôn `company-step.spec.ts` (component thật KHÔNG
 * dựng qua `ng test` — `TestBed.createComponent` treo CPU trong sandbox, đã ghi ở Checkpoint
 * story-05). Chạy qua `npx vitest run -c vitest-base.config.ts <spec>` (hàm thuần + `form()` core
 * `@angular/forms/signals`, không PrimeNG/DOM).
 */
describe('branch-step (logic thuần)', () => {
  it('(a) computeBranchStepIsEmpty = true khi model = emptyBranchStep()', () => {
    expect(computeBranchStepIsEmpty(emptyBranchStep())).toBe(true);
  });

  it('(b) computeBranchStepIsEmpty = false khi user đổi 1 field khỏi mặc định (name)', () => {
    const m: BranchStepModel = { ...emptyBranchStep(), name: 'Chi nhánh 1' };
    expect(computeBranchStepIsEmpty(m)).toBe(false);
  });

  it('(c) computeBranchStepIsEmpty = false khi chỉ có address (whitespace ở field khác vẫn coi là rỗng)', () => {
    const m: BranchStepModel = { ...emptyBranchStep(), address: 'Ha Noi' };
    expect(computeBranchStepIsEmpty(m)).toBe(false);
    // Field chỉ toàn khoảng trắng vẫn coi là "rỗng" (trim) — không phải "đã nhập".
    const m2: BranchStepModel = { ...emptyBranchStep(), name: '   ', hotline: '  ' };
    expect(computeBranchStepIsEmpty(m2)).toBe(true);
  });

  it('(d) buildBranchStepValue trả đúng shape DTO BE (AC3): address luôn CHUỖI (không null)', () => {
    const m: BranchStepModel = {
      name: 'Chi nhánh 1',
      code: 'CN1',
      address: 'Ha Noi',
      hotline: '0900000000',
      email: 'a@b.com',
    };
    expect(buildBranchStepValue(m)).toEqual({
      name: 'Chi nhánh 1',
      code: 'CN1',
      address: 'Ha Noi',
      hotline: '0900000000',
      email: 'a@b.com',
    });
  });

  it('(e) buildBranchStepValue: address null (chưa nhập) → chuỗi rỗng, KHÔNG có companyId/id', () => {
    const v = buildBranchStepValue(emptyBranchStep());
    expect(v).toEqual({ name: '', code: '', address: '', hotline: '', email: '' });
    expect(v).not.toHaveProperty('companyId');
    expect(v).not.toHaveProperty('id');
  });

  it('(f) valid() (qua form() thật + buildBranchStepSchema) = false khi thiếu address', () => {
    const model = signal<BranchStepModel>({ ...emptyBranchStep(), name: 'Chi nhánh 1', code: 'CN1' });
    const rootField = TestBed.runInInjectionContext(() => form(model, buildBranchStepSchema));
    expect(rootField().invalid()).toBe(true);
  });

  it('(g) valid() (qua form() thật + buildBranchStepSchema) = true khi đủ name/code/address, email hợp lệ', () => {
    const model = signal<BranchStepModel>({
      name: 'Chi nhánh 1',
      code: 'CN1',
      address: 'Ha Noi',
      hotline: '0900000000',
      email: 'a@b.com',
    });
    const rootField = TestBed.runInInjectionContext(() => form(model, buildBranchStepSchema));
    expect(rootField().invalid()).toBe(false);
  });

  it('(h) valid() = false khi email sai định dạng', () => {
    const model = signal<BranchStepModel>({
      name: 'Chi nhánh 1',
      code: 'CN1',
      address: 'Ha Noi',
      hotline: '',
      email: 'khong-phai-email',
    });
    const rootField = TestBed.runInInjectionContext(() => form(model, buildBranchStepSchema));
    expect(rootField().invalid()).toBe(true);
  });

  it('(i) valid() = true khi email rỗng (không bắt buộc)', () => {
    const model = signal<BranchStepModel>({
      name: 'Chi nhánh 1',
      code: 'CN1',
      address: 'Ha Noi',
      hotline: '',
      email: '',
    });
    const rootField = TestBed.runInInjectionContext(() => form(model, buildBranchStepSchema));
    expect(rootField().invalid()).toBe(false);
  });
});
