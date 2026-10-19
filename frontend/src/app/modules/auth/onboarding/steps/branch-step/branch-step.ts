import { ChangeDetectionStrategy, Component, effect, inject, input, signal } from '@angular/core';
import {
  emailError,
  FieldTree,
  form,
  FormField,
  required,
  SchemaPath,
  SchemaPathTree,
  validate,
} from '@angular/forms/signals';
import { TranslatePipe } from '@ngx-translate/core';
import { FloatLabel } from 'primeng/floatlabel';
import { Fluid } from 'primeng/fluid';
import { InputTextModule } from 'primeng/inputtext';
import { KeyFilterModule } from 'primeng/keyfilter';
import { Textarea } from 'primeng/textarea';

import { UtilsService } from '@/share/services/utils.service';

import { OnboardingStepComponent } from '../onboarding-step';

/**
 * Model UI-side bước "Chi nhánh" (story-06 AC1). `address` bind `<textarea pTextarea>` → seed `null`
 * (KHÔNG dính gotcha #9 `building-angular-signal-forms/SKILL.md` — chỉ `<input pInputText>` mới coi
 * `null` là field số); `name`/`code`/`hotline`/`email` bind `<input pInputText type="text">` → seed
 * `''` (gotcha #9), đúng khuôn `emptyBranchModel()` (`branch-form.ts:37-53`).
 */
export interface BranchStepModel {
  name: string;
  code: string;
  address: string | null;
  hotline: string;
  email: string;
}

/**
 * Giá trị `value()` trả cho shell (AC3) — key JSON khớp `dto.BranchInput`
 * (`backend/internal/modules/webapp/config/onboarding/dto/dto.go`). `address` luôn CHUỖI (không
 * `null`) khi gửi lên BE — `dto.BranchInput.Address` là `string`, không phải con trỏ.
 */
export interface BranchStepValue {
  name: string;
  code: string;
  address: string;
  hotline: string;
  email: string;
}

/** Export riêng cho spec (`branch-step.spec.ts`) — hàm thuần, không cần render component. */
export function emptyBranchStep(): BranchStepModel {
  return { name: '', code: '', address: null, hotline: '', email: '' };
}

/** Logic thuần của `value()` — tách khỏi component, cùng lý do `company-step.ts` (`ng test` treo CPU). */
export function buildBranchStepValue(m: BranchStepModel): BranchStepValue {
  return {
    name: m.name,
    code: m.code,
    address: m.address ?? '',
    hotline: m.hotline,
    email: m.email,
  };
}

/**
 * Logic thuần của `isEmpty()` — Dev Notes story-06 (user chốt phương án A): "trống" = name, code,
 * address, hotline, email đều rỗng (trim).
 */
export function computeBranchStepIsEmpty(m: BranchStepModel): boolean {
  return (
    m.name.trim() === '' &&
    m.code.trim() === '' &&
    (m.address ?? '').trim() === '' &&
    m.hotline.trim() === '' &&
    m.email.trim() === ''
  );
}

/**
 * Schema Signal Forms (AC1/AC2) — tách hàm để spec dựng lại `form()` trực tiếp mà không cần
 * component. Validator email khuôn `branch-form.ts:250` (`validate(p.email!, …)`), dùng
 * `inject(UtilsService)` (an toàn trong injection context của `form()`/`TestBed.runInInjectionContext`
 * — `UtilsService` là `providedIn: 'root'`, không cần provider riêng ở spec).
 */
export function buildBranchStepSchema(p: SchemaPathTree<BranchStepModel>): void {
  const utilsService = inject(UtilsService);
  required(p.name!);
  required(p.code!);
  required(p.address!);
  validate(p.email! as SchemaPath<string>, ({ value }) => {
    const v = value();
    return !v || utilsService.isEmail(v) ? null : emailError();
  });
}

/**
 * Bước 2 "Chi nhánh" (story-06; `optional: false` từ 2026-09-19 — xem `step-registry.ts`, đảo AC6
 * gốc) — form Signal Forms nhúng RIÊNG, KHÔNG nhúng `BranchForm` (dùng cho màn quản trị chi nhánh
 * chung, không hợp bối cảnh onboarding). KHÔNG có chọn thương hiệu/`BranchFormControl` (AC1).
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  standalone: true,
  selector: 'app-onboarding-branch-step',
  imports: [TranslatePipe, FormField, InputTextModule, KeyFilterModule, FloatLabel, Fluid, Textarea],
  templateUrl: './branch-step.html',
})
export class OnboardingBranchStep implements OnboardingStepComponent {
  /** Giá trị đã lưu trong `draft['branch']` khi shell render lại bước này (AC6 story-04). */
  readonly initialValue = input<unknown>();

  private readonly model = signal<BranchStepModel>(emptyBranchStep());
  protected readonly rootField = form(this.model, buildBranchStepSchema);

  protected readonly blockSpace: RegExp = /[^\s]/;

  constructor() {
    // Cùng lý do `company-step.ts` (constructor, KHÔNG effect ngoài constructor): `NgComponentOutlet`
    // huỷ/tạo MỚI component khi đổi component type giữa các bước — instance mới đọc `initialValue()`
    // 1 lần là đủ để phục hồi model khi quay lại bước này.
    effect(() => {
      const v = this.initialValue() as Partial<BranchStepValue> | undefined;
      this.model.set({
        name: typeof v?.name === 'string' ? v.name : '',
        code: typeof v?.code === 'string' ? v.code : '',
        address: typeof v?.address === 'string' && v.address ? v.address : null,
        hotline: typeof v?.hotline === 'string' ? v.hotline : '',
        email: typeof v?.email === 'string' ? v.email : '',
      });
    });
  }

  protected isFieldInvalid<V>(field: FieldTree<V>): boolean {
    return field().invalid();
  }

  valid(): boolean {
    return !this.rootField().invalid();
  }

  markTouched(): void {
    this.rootField().markAsTouched();
  }

  value(): BranchStepValue {
    return buildBranchStepValue(this.model());
  }

  isEmpty(): boolean {
    return computeBranchStepIsEmpty(this.model());
  }
}
