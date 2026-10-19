import { TestBed } from '@angular/core/testing';
import { provideTranslateService } from '@ngx-translate/core';
import { of } from 'rxjs';
import { beforeEach, describe, expect, it, Mock, vi } from 'vitest';

import { AuthService } from '@/core/services/auth';
import { OnboardingService } from '../onboarding.service';
import { OnboardingCompanyStep } from '../steps/company-step/company-step';
import { OnboardingStepComponent, OnboardingStepDef } from '../steps/onboarding-step';
import { ONBOARDING_STEPS } from '../steps/step-registry';
import { OnboardingWizard, resolveStepDone } from './onboarding-wizard';

/**
 * 2026-09-19: `branch` đổi `optional: true` → `false` (đảo AC6 gốc — xem `step-registry.ts`), nên
 * registry THẬT không còn bước optional nào để test nhánh "optional + isEmpty() ⇒ auto-skip" của
 * `next()`/`finish()`. Cơ chế `optional`/`skip()` vẫn là hạ tầng chung hợp lệ cho bước tương lai
 * (AC4/AC12: thêm bước chỉ bằng khai báo registry), nên test (b) dưới đây KHÔNG xoá — đổi sang dựng
 * 1 registry giả cục bộ có bước cuối optional, ghi đè `wizard.steps` (cùng khuôn cast `unknown` đã
 * dùng cho `activeIndex`/`skipped`/`draft` trong file này) để tách khỏi cấu hình nghiệp vụ thật.
 * `component` không được `NgComponentOutlet` thật sự dựng (test stub `outletRef` thẳng), tái dùng
 * `OnboardingCompanyStep` chỉ để thoả kiểu `Type<OnboardingStepComponent>`.
 */
const FAKE_STEPS_WITH_OPTIONAL_LAST: OnboardingStepDef[] = [
  { key: 'company', titleKey: 'x', optional: false, component: OnboardingCompanyStep },
  { key: 'x_fake_optional', titleKey: 'x', optional: true, component: OnboardingCompanyStep },
];

/**
 * QA pass 1 (`Review Triage Log`, finding [ghi nhận] "0 spec cho toàn bộ onboarding/") — lưới tối
 * thiểu cho logic SHELL (`next/skip/finish/commitCurrentStep`), KHÔNG phải cho từng step component.
 *
 * QA pass 2 ac_gap (user chốt phương án A, `Story Change Log` AC5b): "trống" (`isEmpty()`) ≠
 * "invalid" — bước optional có thể vừa KHÔNG trống vừa `!valid()` (nhập dở dang), lúc đó PHẢI bị
 * chặn như bước bắt buộc, không được âm thầm coi là "đã bỏ qua". Spec (b) sửa lại theo đúng nghĩa
 * "trống" (`isEmpty()` true), thêm (d)/(e) phủ đúng ac_gap.
 *
 * QA pass 3 story_defect: điều kiện auto-skip của `finish()` từng có thêm vế
 * `|| skipped().has(key)` (bản pass 2) — bug data-loss mới: bước từng bị skip mà quay lại nhập HỢP
 * LỆ vẫn bị coi là "vẫn bỏ qua" vĩnh viễn. Đã bỏ vế đó; spec (f) khoá lại đúng AC6 (bước đã bỏ qua mà
 * quay lại nhập ⇒ gỡ khỏi `skipped`, dùng dữ liệu mới).
 *
 * Dựng bằng `TestBed.runInInjectionContext(() => new OnboardingWizard())` (khuôn có sẵn trong
 * `appointment-workspace.deeplink.spec.ts`) — KHÔNG `TestBed.createComponent` + `detectChanges()`:
 * shell không cần render `p-stepper`/`NgComponentOutlet` thật để verify logic điều hướng, và
 * `viewChild(NgComponentOutlet)` chỉ resolve sau 1 lượt view render mà ta cố tình bỏ qua. Thay vào
 * đó stub thẳng field riêng `outletRef` bằng 1 `OnboardingStepComponent` giả — đúng tinh thần contract
 * (`OnboardingStepDef`/`OnboardingStepComponent`) mà shell khai báo: nó không quan tâm bước THẬT là gì,
 * chỉ gọi `valid()/markTouched()/value()/isEmpty()`. Đây là hàm thuần/service theo executor-contract
 * §2, chạy qua `vitest run -c vitest-base.config.ts`, KHÔNG cần `ng test`.
 */
function stubOutlet(wizard: OnboardingWizard, instance: OnboardingStepComponent | null): void {
  (wizard as unknown as { outletRef: () => { componentInstance: OnboardingStepComponent | null } | undefined }).outletRef =
    () => (instance ? { componentInstance: instance } : undefined);
}

/** Mặc định: hợp lệ, KHÔNG trống (đại diện 1 bước "bình thường đã điền xong"). */
// `markTouched` bị loại khỏi `overrides`: luôn là spy (test assert số lần gọi) — cho override bằng
// hàm thường thì kiểu trả về không còn là `Mock` (TS2322 dưới vitest 4).
function fakeStep(overrides: Partial<Omit<OnboardingStepComponent, 'markTouched'>> = {}): OnboardingStepComponent & {
  markTouched: Mock<() => void>;
} {
  return {
    valid: () => true,
    markTouched: vi.fn<() => void>(),
    value: () => '',
    isEmpty: () => false,
    ...overrides,
  };
}

describe('OnboardingWizard (shell logic)', () => {
  let bootstrapSpy: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    bootstrapSpy = vi.fn().mockReturnValue(of({ branch_id: null }));
    TestBed.configureTestingModule({
      providers: [
        provideTranslateService(),
        { provide: OnboardingService, useValue: { status: vi.fn(), bootstrap: bootstrapSpy } },
        // story-43: OnboardingWizard nay gọi authService.refreshBusinessProfile() sau bootstrap —
        // fake tối thiểu, spec không quan tâm hồ sơ doanh nghiệp.
        { provide: AuthService, useValue: { refreshBusinessProfile: vi.fn().mockResolvedValue(undefined) } },
      ],
    });
  });

  it('(a) bước bắt buộc (company) invalid ⇒ "Tiếp" gọi markTouched, KHÔNG chuyển bước', () => {
    const wizard = TestBed.runInInjectionContext(() => new OnboardingWizard());
    expect(ONBOARDING_STEPS[0].optional).toBe(false); // đối chứng: bước 0 THẬT SỰ bắt buộc
    const step = fakeStep({ valid: () => false, isEmpty: () => true });
    stubOutlet(wizard, step);

    (wizard as unknown as { next: () => void }).next();

    expect(step.markTouched).toHaveBeenCalledTimes(1);
    expect((wizard as unknown as { activeIndex: () => number }).activeIndex()).toBe(0);
    expect(bootstrapSpy).not.toHaveBeenCalled();
  });

  it('(b) bước cuối optional (registry giả) THẬT SỰ TRỐNG ⇒ "Hoàn tất" vẫn gọi bootstrap, key vào skipped', () => {
    const wizard = TestBed.runInInjectionContext(() => new OnboardingWizard());
    (wizard as unknown as { steps: OnboardingStepDef[] }).steps = FAKE_STEPS_WITH_OPTIONAL_LAST;
    const lastIndex = FAKE_STEPS_WITH_OPTIONAL_LAST.length - 1;
    (wizard as unknown as { activeIndex: { set: (v: number) => void } }).activeIndex.set(lastIndex);
    // "trống" = isEmpty() true — user CHƯA nhập gì (khác (d): có nhập nhưng invalid).
    const step = fakeStep({ valid: () => false, isEmpty: () => true });
    stubOutlet(wizard, step);

    (wizard as unknown as { finish: () => void }).finish();

    expect(step.markTouched).not.toHaveBeenCalled();
    expect(bootstrapSpy).toHaveBeenCalledTimes(1);
    const key = FAKE_STEPS_WITH_OPTIONAL_LAST[lastIndex].key;
    expect((wizard as unknown as { skipped: () => Set<string> }).skipped().has(key)).toBe(true);
  });

  it('(c) Quay lại rồi tiến tiếp ⇒ giữ nguyên draft đã lưu; bỏ qua rồi nhập lại thì gỡ khỏi skipped', () => {
    const wizard = TestBed.runInInjectionContext(() => new OnboardingWizard());
    const companyKey = ONBOARDING_STEPS[0].key;
    (wizard as unknown as { skipped: { set: (v: Set<string>) => void } }).skipped.set(new Set([companyKey]));

    const step = fakeStep({ valid: () => true, value: () => 'Acme', isEmpty: () => false });
    stubOutlet(wizard, step);

    (wizard as unknown as { next: () => void }).next(); // company → branch, commit 'Acme'
    expect((wizard as unknown as { activeIndex: () => number }).activeIndex()).toBe(1);
    expect((wizard as unknown as { skipped: () => Set<string> }).skipped().has(companyKey)).toBe(false);

    (wizard as unknown as { back: () => void }).back(); // branch → company
    expect((wizard as unknown as { activeIndex: () => number }).activeIndex()).toBe(0);
    expect(
      (wizard as unknown as { stepInputs: () => { initialValue: unknown } }).stepInputs().initialValue,
    ).toBe('Acme');

    (wizard as unknown as { next: () => void }).next(); // company → branch lại
    expect((wizard as unknown as { activeIndex: () => number }).activeIndex()).toBe(1);
    expect((wizard as unknown as { draft: () => Record<string, unknown> }).draft()[companyKey]).toBe('Acme');
  });

  it('(d) bước cuối optional (branch) nhập DỞ (không trống nhưng invalid) ⇒ "Hoàn tất" KHÔNG gọi bootstrap, chặn + markTouched', () => {
    const wizard = TestBed.runInInjectionContext(() => new OnboardingWizard());
    const lastIndex = ONBOARDING_STEPS.length - 1;
    (wizard as unknown as { activeIndex: { set: (v: number) => void } }).activeIndex.set(lastIndex);
    // Khác (b): CÓ nhập (isEmpty() false) nhưng vẫn invalid — ac_gap pass 2, KHÔNG được coi là bỏ qua.
    const step = fakeStep({ valid: () => false, isEmpty: () => false });
    stubOutlet(wizard, step);

    (wizard as unknown as { finish: () => void }).finish();

    expect(step.markTouched).toHaveBeenCalledTimes(1);
    expect(bootstrapSpy).not.toHaveBeenCalled();
    const key = ONBOARDING_STEPS[lastIndex].key;
    expect((wizard as unknown as { skipped: () => Set<string> }).skipped().has(key)).toBe(false);
  });

  it('(e) nhập DỞ rồi bấm "Bỏ qua" ở bước cuối ⇒ gọi bootstrap, key vào skipped, draft[key] bị xoá', () => {
    const wizard = TestBed.runInInjectionContext(() => new OnboardingWizard());
    const lastIndex = ONBOARDING_STEPS.length - 1;
    const key = ONBOARDING_STEPS[lastIndex].key;
    (wizard as unknown as { activeIndex: { set: (v: number) => void } }).activeIndex.set(lastIndex);
    // Giả lập draft đã có dữ liệu dở dang từ trước (vd user gõ rồi quay lại) — skip() phải xoá nó.
    (wizard as unknown as { draft: { set: (v: Record<string, unknown>) => void } }).draft.set({
      [key]: 'nhập dở dang',
    });
    const step = fakeStep({ valid: () => false, isEmpty: () => false });
    stubOutlet(wizard, step);

    (wizard as unknown as { skip: () => void }).skip();

    expect(bootstrapSpy).toHaveBeenCalledTimes(1);
    expect((wizard as unknown as { skipped: () => Set<string> }).skipped().has(key)).toBe(true);
    expect((wizard as unknown as { draft: () => Record<string, unknown> }).draft()[key]).toBeUndefined();
  });

  it('(f) đã Bỏ qua bước cuối, quay lại nhập HỢP LỆ rồi Hoàn tất ⇒ dùng dữ liệu mới, gỡ khỏi skipped', () => {
    // QA pass 3 story_defect: bản pass 2 có thêm `|| skipped().has(key)` vào điều kiện auto-skip của
    // finish() ⇒ bước từng bị skip mà quay lại nhập HỢP LỆ vẫn bị coi là "vẫn bỏ qua" vĩnh viễn, dữ
    // liệu mới bị vứt bỏ âm thầm — vi phạm AC6 ("bước đã bỏ qua mà quay lại nhập ⇒ gỡ khỏi skipped").
    const wizard = TestBed.runInInjectionContext(() => new OnboardingWizard());
    const lastIndex = ONBOARDING_STEPS.length - 1;
    const key = ONBOARDING_STEPS[lastIndex].key;
    (wizard as unknown as { activeIndex: { set: (v: number) => void } }).activeIndex.set(lastIndex);
    // Giả lập: bước cuối đã từng bị "Bỏ qua" trước đó (key còn trong `skipped` từ 1 lượt tương tác cũ).
    (wizard as unknown as { skipped: { set: (v: Set<string>) => void } }).skipped.set(new Set([key]));
    // Nhưng NGAY LÚC NÀY, instance đang render là bước user vừa nhập lại HỢP LỆ, KHÔNG còn trống.
    const step = fakeStep({ valid: () => true, isEmpty: () => false, value: () => 'Chi nhánh mới' });
    stubOutlet(wizard, step);

    (wizard as unknown as { finish: () => void }).finish();

    expect(bootstrapSpy).toHaveBeenCalledTimes(1);
    expect((wizard as unknown as { skipped: () => Set<string> }).skipped().has(key)).toBe(false);
    expect((wizard as unknown as { draft: () => Record<string, unknown> }).draft()[key]).toBe(
      'Chi nhánh mới',
    );
  });

  /**
   * Story-06 AC3 — `buildBootstrapFormData()` thật (thay payload tạm story-04). Gọi trực tiếp qua
   * cast `unknown` (method `private`) — cùng khuôn test (b)-(f) ở trên đã cast field private/protected.
   */
  it('(g) buildBootstrapFormData: company JSON KHÔNG chứa logoFile, logoFile append thành FormData entry riêng', () => {
    const wizard = TestBed.runInInjectionContext(() => new OnboardingWizard());
    const file = new File(['x'], 'logo.png', { type: 'image/png' });
    (wizard as unknown as { draft: { set: (v: Record<string, unknown>) => void } }).draft.set({
      company: { name: 'Acme', code: 'ACM', logoFile: file },
    });
    (wizard as unknown as { skipped: { set: (v: Set<string>) => void } }).skipped.set(new Set(['branch']));

    const fd = (wizard as unknown as { buildBootstrapFormData: () => FormData }).buildBootstrapFormData();

    const companyJson = JSON.parse(fd.get('company') as string);
    expect(companyJson).not.toHaveProperty('logoFile');
    expect(companyJson).toEqual({ name: 'Acme', code: 'ACM' });
    expect(fd.get('logoFile')).toBe(file);
    expect(fd.get('branch')).toBeNull();
  });

  it('(h) buildBootstrapFormData: branch KHÔNG gửi khi key nằm trong `skipped`, dù `draft` còn giá trị cũ (mutation-verify target: "gửi branch dù đã skip")', () => {
    const wizard = TestBed.runInInjectionContext(() => new OnboardingWizard());
    (wizard as unknown as { draft: { set: (v: Record<string, unknown>) => void } }).draft.set({
      company: { name: 'Acme' },
      branch: { name: 'CN cũ' },
    });
    (wizard as unknown as { skipped: { set: (v: Set<string>) => void } }).skipped.set(new Set(['branch']));

    const fd = (wizard as unknown as { buildBootstrapFormData: () => FormData }).buildBootstrapFormData();

    expect(fd.get('branch')).toBeNull();
  });

  it('(i) buildBootstrapFormData: branch gửi đúng JSON khi KHÔNG bị skip; KHÔNG có logoFile khi company không có logo', () => {
    const wizard = TestBed.runInInjectionContext(() => new OnboardingWizard());
    (wizard as unknown as { draft: { set: (v: Record<string, unknown>) => void } }).draft.set({
      company: { name: 'Acme' },
      branch: { name: 'CN 1', code: 'CN1', address: 'HN', hotline: '', email: '' },
    });

    const fd = (wizard as unknown as { buildBootstrapFormData: () => FormData }).buildBootstrapFormData();

    expect(JSON.parse(fd.get('branch') as string)).toEqual({
      name: 'CN 1',
      code: 'CN1',
      address: 'HN',
      hotline: '',
      email: '',
    });
    expect(fd.get('logoFile')).toBeNull();
  });
});

/**
 * QA pass 1 story-08 (P1/P2) — `resolveStepDone()` là hàm thuần export riêng (tách khỏi
 * `OnboardingWizard.isStepDone`, khuôn `resolvePostLoginTarget` ở `syncing.ts`), test trực tiếp
 * KHÔNG cần dựng component. Case (b) khoá đúng P1: bước ĐANG LÀM mà vẫn còn trong `skipped` (kịch
 * bản bước cuối optional bấm "Bỏ qua" rồi submit lỗi — `error` callback không xoá `skipped`/không đổi
 * `activeIndex`) PHẢI trả `false`, KHÔNG được hiện `pi-check` như đã xong.
 */
describe('resolveStepDone (logic thuần, story-08 QA pass 1 P1/P2)', () => {
  it('(a) index < activeIndex ⇒ true (bước đã đi qua)', () => {
    expect(resolveStepDone(0, 1, 'company', new Set())).toBe(true);
  });

  it('(b) index === activeIndex dù key nằm trong skipped ⇒ false (P1 — bước ĐANG LÀM không được coi là đã xong)', () => {
    expect(resolveStepDone(1, 1, 'branch', new Set(['branch']))).toBe(false);
  });

  it('(c) index > activeIndex và key đã skip ⇒ true', () => {
    expect(resolveStepDone(1, 0, 'branch', new Set(['branch']))).toBe(true);
  });

  it('(d) index > activeIndex và key CHƯA skip ⇒ false', () => {
    expect(resolveStepDone(1, 0, 'branch', new Set())).toBe(false);
  });
});
