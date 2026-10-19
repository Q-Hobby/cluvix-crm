import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { form } from '@angular/forms/signals';
import { describe, expect, it } from 'vitest';

import {
  buildCompanyStepSchema,
  buildCompanyStepValue,
  CompanyStepModel,
  computeCompanyStepIsEmpty,
  emptyCompanyStep,
  restoreCompanyStepModel,
} from './company-step';

/**
 * Story-05 — lưới tối thiểu cho logic THUẦN của bước "Thương hiệu": `isEmpty()`/`value()`/`valid()`
 * (theo contract `OnboardingStepComponent`). Story-08 AC3/AC9 đã bỏ field `weekly_rest_days` khỏi
 * model — test (e) convert CSV weekday cũ bị xoá theo vì hàm `csvToWeekdays`/`weekdaysToCsv` không
 * còn tồn tại.
 *
 * KHÔNG dựng `OnboardingCompanyStep` qua `TestBed.createComponent` + `detectChanges()`: đã thử
 * (`ng test --include=...`) và bị TREO CPU 100% không dứt trong sandbox — nghi đo đạc DOM của
 * `p-select`/`p-fileupload` (PrimeNG) trong jsdom không bao giờ ổn định để `whenStable()`/effect
 * flush hoàn tất, KHÔNG xác định được nguyên nhân chắc chắn trong thời gian cho phép của story này
 * (ghi vào Checkpoint — nợ cho ai làm bước sau muốn render component này qua `ng test`). Giải pháp:
 * tách logic `value()`/`isEmpty()` thành hàm thuần `buildCompanyStepValue`/`computeCompanyStepIsEmpty`
 * (không cần Angular component/DI) — component chỉ là wrapper mỏng gọi lại đúng các hàm này (xem
 * `company-step.ts`). `valid()` test bằng cách dựng LẠI `form()` thật (cùng `buildCompanyStepSchema`
 * component dùng) qua `TestBed.runInInjectionContext`, KHÔNG cần view/template — không dính hang vì
 * không có PrimeNG/DOM liên quan, chỉ core `@angular/forms/signals`.
 *
 * Chạy qua `npx vitest run -c vitest-base.config.ts <spec>` (hàm thuần + `form()` không cần compile
 * template Angular — đúng nhánh "hàm thuần/service" của executor-contract §2, KHÔNG cần `ng test`).
 */
describe('company-step (logic thuần)', () => {
  it('(a) computeCompanyStepIsEmpty = true khi model = emptyCompanyStep() + không có logo — default timezone/currency KHÔNG tính là đã nhập', () => {
    expect(computeCompanyStepIsEmpty(emptyCompanyStep(), null)).toBe(true);
  });

  it('(b) computeCompanyStepIsEmpty = false khi đã chọn logo, mọi field khác vẫn mặc định', () => {
    const file = new File(['x'], 'logo.png', { type: 'image/png' });
    expect(computeCompanyStepIsEmpty(emptyCompanyStep(), file)).toBe(false);
  });

  it('(c) computeCompanyStepIsEmpty = false khi user đổi 1 field khỏi mặc định (name)', () => {
    const m: CompanyStepModel = { ...emptyCompanyStep(), name: 'Acme' };
    expect(computeCompanyStepIsEmpty(m, null)).toBe(false);
  });

  it('(c2) computeCompanyStepIsEmpty = false khi user đổi timezone/currency khỏi mặc định (chưa nhập field nào khác)', () => {
    const m: CompanyStepModel = { ...emptyCompanyStep(), timezone: 'Asia/Tokyo', currency: 'USD' };
    expect(computeCompanyStepIsEmpty(m, null)).toBe(false);
  });

  it('(d) buildCompanyStepValue trả đúng shape DTO BE (story-08 AC3: KHÔNG còn weekly_rest_days), logoFile tách riêng, KHÔNG có allowed_ips/require_2fa/id', () => {
    const m: CompanyStepModel = {
      name: 'Acme',
      code: 'ACM',
      business_type_id: 3,
      timezone: 'Asia/Tokyo',
      currency: 'USD',
      currency_symbol: '$',
      currency_locale: 'en-US',
      hotline: '0900000000',
      address: 'Ha Noi',
      email: 'a@b.com',
    };
    const file = new File(['x'], 'logo.png', { type: 'image/png' });
    const v = buildCompanyStepValue(m, file);

    expect(v).toEqual({
      name: 'Acme',
      code: 'ACM',
      business_type_id: 3,
      timezone: 'Asia/Tokyo',
      currency: 'USD',
      currency_symbol: '$',
      currency_locale: 'en-US',
      hotline: '0900000000',
      address: 'Ha Noi',
      email: 'a@b.com',
      logoFile: file,
    });
    expect(v).not.toHaveProperty('weekly_rest_days');
    expect(v).not.toHaveProperty('allowed_ips');
    expect(v).not.toHaveProperty('require_2fa');
    expect(v).not.toHaveProperty('require_2fa_roles');
    expect(v).not.toHaveProperty('id');
  });

  it('(f) valid() (qua form() thật + buildCompanyStepSchema) = false khi thiếu business_type_id', () => {
    const model = signal<CompanyStepModel>({ ...emptyCompanyStep(), name: 'Acme', code: 'ACM' });
    const rootField = TestBed.runInInjectionContext(() => form(model, buildCompanyStepSchema));
    expect(rootField().invalid()).toBe(true);
  });

  it('(g) valid() (qua form() thật + buildCompanyStepSchema) = true khi đủ 3 field bắt buộc', () => {
    const model = signal<CompanyStepModel>({
      ...emptyCompanyStep(),
      name: 'Acme',
      code: 'ACM',
      business_type_id: 3,
    });
    const rootField = TestBed.runInInjectionContext(() => form(model, buildCompanyStepSchema));
    expect(rootField().invalid()).toBe(false);
  });

  it('(h) story-11 AC2 — restoreCompanyStepModel GIỮ NGUYÊN "" user xoá trắng currency_symbol/currency_locale, KHÔNG hiện lại DEFAULT', () => {
    const m = restoreCompanyStepModel({
      name: 'Acme',
      code: 'ACM',
      business_type_id: 3,
      timezone: 'Asia/Ho_Chi_Minh',
      currency: 'VND',
      currency_symbol: '',
      currency_locale: '',
      hotline: '0900000000',
      address: null,
      email: 'a@b.com',
    });
    expect(m.currency_symbol).toBe('');
    expect(m.currency_locale).toBe('');
  });

  it('(i) restoreCompanyStepModel dùng DEFAULT khi currency_symbol/currency_locale VẮNG MẶT (undefined) — khác ca (h) là "" cố ý', () => {
    const m = restoreCompanyStepModel({
      name: 'Acme',
      code: 'ACM',
      business_type_id: 3,
    });
    expect(m.currency_symbol).toBe(emptyCompanyStep().currency_symbol);
    expect(m.currency_locale).toBe(emptyCompanyStep().currency_locale);
  });

  it('(j) restoreCompanyStepModel dùng DEFAULT khi initialValue = undefined (bước 1 chưa có draft)', () => {
    const m = restoreCompanyStepModel(undefined);
    expect(m).toEqual(emptyCompanyStep());
  });
});
