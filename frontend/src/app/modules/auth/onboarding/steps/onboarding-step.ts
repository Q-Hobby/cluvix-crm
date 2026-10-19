import { Type } from '@angular/core';

/**
 * Contract mỗi bước wizard onboarding phải hiện thực. Shell (`onboarding-wizard.ts`) chỉ biết
 * 3 hàm này — KHÔNG biết field bên trong bước, để `step-registry.ts` là nơi DUY NHẤT phải sửa khi
 * thêm/bớt bước (AC4).
 */
export interface OnboardingStepComponent {
  /** Shell gọi TRƯỚC khi cho phép "Tiếp" — bước tự validate nội bộ. */
  valid(): boolean;
  /** Shell gọi khi user bấm "Tiếp" mà `valid()` false — bước tự hiện lỗi field. */
  markTouched(): void;
  /** Shell gọi khi rời bước (Tiếp/Hoàn tất) để gom dữ liệu vào `draft[key]`. */
  value(): unknown;
  /**
   * AC5b (QA pass 2 ac_gap, user chốt phương án A) — "trống" = user CHƯA NHẬP GÌ (giá trị mặc định
   * tự điền sẵn KHÔNG tính là đã nhập). Shell chỉ được tự coi bước `optional` là "bỏ qua" khi
   * `isEmpty()` true; nhập DỞ (có gõ nhưng `valid()` vẫn false) PHẢI bị `finish()`/`next()` CHẶN như
   * bước bắt buộc — không được âm thầm bỏ dữ liệu người dùng đã nhập.
   */
  isEmpty(): boolean;
}

/** Khai báo 1 bước trong registry — xem `step-registry.ts`. */
export interface OnboardingStepDef {
  /** Khoá bước, dùng làm key trong `draft`/`skipped` — PHẢI duy nhất trong registry. */
  key: string;
  /** i18n key cho tiêu đề hiện trên header stepper. */
  titleKey: string;
  /**
   * Story-08 AC7 — i18n key cho mô tả ngắn hiện dưới tiêu đề bước ở cột trái layout side-by-side.
   * TUỲ CHỌN (additive, không phá contract story-04) — bước không khai `descKey` thì cột trái chỉ
   * hiện tiêu đề, không có dòng mô tả.
   */
  descKey?: string;
  /** `true` ⇒ shell hiện thêm nút "Bỏ qua" cho bước này. */
  optional: boolean;
  /** Component render nội dung bước, nhận input `initialValue` (giá trị đã lưu trong draft, nếu có). */
  component: Type<OnboardingStepComponent>;
}
