import { ChangeDetectionStrategy, Component, DestroyRef, effect, inject, input, signal } from '@angular/core';
import { FieldTree, form, FormField, required, SchemaPathTree } from '@angular/forms/signals';
import { TranslatePipe, TranslateService } from '@ngx-translate/core';
import { ButtonModule } from 'primeng/button';
import { FileUploadModule } from 'primeng/fileupload';
import { FloatLabel } from 'primeng/floatlabel';
import { Fluid } from 'primeng/fluid';
import { InputTextModule } from 'primeng/inputtext';
import { KeyFilterModule } from 'primeng/keyfilter';
import { Select } from 'primeng/select';
import { Textarea } from 'primeng/textarea';

import {
  BUSINESS_TYPE_OPTIONS,
  CONFIG_FILE_SELECT,
  CURRENCY_OPTIONS,
  DEFAULT_CURRENCY,
  DEFAULT_CURRENCY_LOCALE,
  DEFAULT_CURRENCY_SYMBOL,
  DEFAULT_TIMEZONE,
  TIMEZONE_OPTIONS,
} from '@/define';
import { isAllowExtensions } from '@/share/services/utils.service';

import { OnboardingStepComponent } from '../onboarding-step';

/**
 * Model UI-side bước "Thương hiệu" (story-05 AC1; story-08 AC3 đã bỏ `weekly_rest_days` — user
 * không muốn hỏi ngày nghỉ hằng tuần ở bước onboarding, BE `dto.CompanyInput.WeeklyRestDays` giữ
 * nguyên và tự dùng mặc định khi field không được gửi). Field text bind
 * `<input pInputText type="text">` seed `''` (gotcha #9), field số/reference seed `null`
 * (gotcha #1) — KHÔNG field nào `undefined`.
 */
export interface CompanyStepModel {
  name: string;
  code: string;
  business_type_id: number | null;
  timezone: string;
  currency: string;
  currency_symbol: string;
  currency_locale: string;
  hotline: string;
  address: string | null;
  email: string;
}

/**
 * Giá trị `value()` trả cho shell (AC3) — key JSON khớp `dto.CompanyInput`
 * (`backend/internal/modules/webapp/config/onboarding/dto/dto.go`), KHÔNG gửi `weekly_rest_days`
 * (story-08 AC3 — BE để mặc định khi thiếu field). `logoFile` nằm CẠNH các key JSON (KHÔNG bọc
 * trong 1 sub-object `company`) — story-06 tách `logoFile` ra khỏi phần còn lại rồi
 * `JSON.stringify(...)` phần JSON trước khi append FormData, đúng khuôn `CompanyForm.onSubmit`
 * (`formValue.logoFile = this.logoFile` cạnh field JSON khác).
 */
export interface CompanyStepValue {
  name: string;
  code: string;
  business_type_id: number | null;
  timezone: string;
  currency: string;
  currency_symbol: string;
  currency_locale: string;
  hotline: string;
  address: string | null;
  email: string;
  logoFile: File | null;
}

/** Export riêng cho spec (`company-step.spec.ts`) — hàm thuần, không cần render component. */
export function emptyCompanyStep(): CompanyStepModel {
  return {
    name: '',
    code: '',
    business_type_id: null,
    timezone: DEFAULT_TIMEZONE,
    currency: DEFAULT_CURRENCY,
    currency_symbol: DEFAULT_CURRENCY_SYMBOL,
    currency_locale: DEFAULT_CURRENCY_LOCALE,
    hotline: '',
    address: null,
    email: '',
  };
}

/**
 * Logic thuần của `value()` — tách khỏi component để spec test được KHÔNG cần render (`ng test`
 * dựng `TestBed.createComponent` cho component này bị treo CPU 100% trong sandbox — nghi PrimeNG
 * `p-select`/`p-fileupload` đo đạc DOM trong jsdom không bao giờ ổn định; ghi ở Checkpoint story-05,
 * chưa xác định gốc chắc chắn). Component method chỉ là wrapper mỏng gọi hàm này.
 */
export function buildCompanyStepValue(m: CompanyStepModel, logoFile: File | null): CompanyStepValue {
  return {
    name: m.name,
    code: m.code,
    business_type_id: m.business_type_id,
    timezone: m.timezone,
    currency: m.currency,
    currency_symbol: m.currency_symbol,
    currency_locale: m.currency_locale,
    hotline: m.hotline,
    address: m.address,
    email: m.email,
    logoFile,
  };
}

/**
 * Logic thuần của `isEmpty()` — cùng lý do tách ở trên. AC5b: giá trị mặc định tự điền
 * (timezone/currency/currency_symbol/currency_locale) KHÔNG tính là "đã nhập"; logo đã chọn tính là
 * "đã nhập".
 */
export function computeCompanyStepIsEmpty(m: CompanyStepModel, logoFile: File | null): boolean {
  const empty = emptyCompanyStep();
  return (
    m.name === empty.name &&
    m.code === empty.code &&
    m.business_type_id === empty.business_type_id &&
    m.timezone === empty.timezone &&
    m.currency === empty.currency &&
    m.currency_symbol === empty.currency_symbol &&
    m.currency_locale === empty.currency_locale &&
    m.hotline === empty.hotline &&
    (m.address ?? '') === '' &&
    m.email === empty.email &&
    logoFile === null
  );
}

/**
 * Logic thuần khôi phục `model` từ `draft['company']` khi shell tạo lại instance bước này (AC4/AC6) —
 * tách khỏi `effect()` trong constructor để spec test được ca `''` (story-11 AC2) mà KHÔNG cần render
 * component (cùng lý do các hàm thuần khác trong file — component nặng PrimeNG treo CPU, xem
 * `company-step.spec.ts`).
 *
 * `currency_symbol`/`currency_locale` dùng `??` (chỉ THIẾU KEY hoặc `undefined` mới lấy DEFAULT) —
 * user xoá trắng `''` là giá trị THẬT (cố ý không dùng ký hiệu/locale mặc định), Quay lại rồi Tiếp
 * phải giữ nguyên `''`, KHÔNG hiện lại DEFAULT. Các field khác (`timezone`/`currency`) GIỮ NGUYÊN
 * ngữ nghĩa `typeof … === 'string' && v.x ? v.x : DEFAULT` — đây là 2 field chọn từ danh sách cố định
 * qua `<p-select>`, UI không cho phép user gõ rỗng nên `''` chỉ có thể là dữ liệu hỏng/thiếu, không
 * phải lựa chọn cố ý như 2 field tiền tệ text tự do ở trên.
 */
export function restoreCompanyStepModel(v: Partial<CompanyStepValue> | undefined): CompanyStepModel {
  return {
    name: typeof v?.name === 'string' ? v.name : '',
    code: typeof v?.code === 'string' ? v.code : '',
    business_type_id: v?.business_type_id ?? null,
    timezone: typeof v?.timezone === 'string' && v.timezone ? v.timezone : DEFAULT_TIMEZONE,
    currency: typeof v?.currency === 'string' && v.currency ? v.currency : DEFAULT_CURRENCY,
    currency_symbol: v?.currency_symbol ?? DEFAULT_CURRENCY_SYMBOL,
    currency_locale: v?.currency_locale ?? DEFAULT_CURRENCY_LOCALE,
    hotline: typeof v?.hotline === 'string' ? v.hotline : '',
    address: v?.address ?? null,
    email: typeof v?.email === 'string' ? v.email : '',
  };
}

/** Schema Signal Forms (AC2) — tách hàm để spec dựng lại `form()` trực tiếp mà không cần component. */
export function buildCompanyStepSchema(p: SchemaPathTree<CompanyStepModel>): void {
  required(p.name!);
  required(p.code!);
  required(p.business_type_id!);
}

/**
 * Bước 1 "Thương hiệu" (story-05) — form Signal Forms nhúng RIÊNG, KHÔNG nhúng `CompanyForm`
 * (architecture.md §Thiết kế FE: `CompanyForm.ngOnInit` gọi `rbacService.getChildRoleForDropdown()`
 * không miễn gate ⇒ 403 ⇒ interceptor redirect, phụ thuộc `dialogRef`, mang field 2FA/IP không
 * thuộc wizard). Bước NÀY BẮT BUỘC (`optional: false` trong `step-registry.ts`) nhưng vẫn cài
 * `isEmpty()` theo contract `OnboardingStepComponent`.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  standalone: true,
  selector: 'app-onboarding-company-step',
  imports: [
    TranslatePipe,
    FormField,
    InputTextModule,
    KeyFilterModule,
    Select,
    FloatLabel,
    Fluid,
    Textarea,
    FileUploadModule,
    ButtonModule,
  ],
  templateUrl: './company-step.html',
})
export class OnboardingCompanyStep implements OnboardingStepComponent {
  /** Giá trị đã lưu trong `draft['company']` khi shell render lại bước này (AC4). */
  readonly initialValue = input<unknown>();

  private readonly destroyRef = inject(DestroyRef);
  private readonly translate = inject(TranslateService);

  private readonly model = signal<CompanyStepModel>(emptyCompanyStep());
  protected readonly rootField = form(this.model, buildCompanyStepSchema);

  protected readonly TIMEZONE_OPTIONS = TIMEZONE_OPTIONS;
  protected readonly CURRENCY_OPTIONS = CURRENCY_OPTIONS;
  protected readonly BUSINESS_TYPE_OPTIONS = BUSINESS_TYPE_OPTIONS;
  protected readonly CONFIG_FILE_SELECT = CONFIG_FILE_SELECT;
  protected readonly blockSpace: RegExp = /[^\s]/;

  /** Logo — SỐNG NGOÀI `form()` (không phải field JSON gửi thẳng, đúng pattern `CompanyForm.logoFile`). */
  logoFile: File | null = null;
  protected readonly logoError = signal<string | null>(null);
  protected readonly logoPreviewUrl = signal<string | null>(null);

  constructor() {
    // `initialValue` chỉ có giá trị THẬT ngay sau khi shell tạo lại instance này (quay lại bước 1
    // từ bước 2 — `NgComponentOutlet` huỷ/tạo MỚI component khi đổi component type giữa các bước,
    // khác hẳn `placeholder-step` dùng CHUNG 1 class cho 2 bước nên còn cần effect() phản ứng theo
    // từng lần đổi input). Effect ở constructor đủ để phục hồi model + logo cho instance mới.
    effect(() => {
      const v = this.initialValue() as Partial<CompanyStepValue> | undefined;
      this.model.set(restoreCompanyStepModel(v));
      this.setLogoFile(v?.logoFile ?? null);
    });

    this.destroyRef.onDestroy(() => this.revokeLogoPreview());
  }

  protected isFieldInvalid<V>(field: FieldTree<V>): boolean {
    return field().invalid();
  }

  /** Đổi tiền tệ → tự điền symbol/locale (đối chiếu `CompanyForm.onCurrencyChange`). */
  protected onCurrencyChange(currencyCode: string): void {
    const opt = CURRENCY_OPTIONS.find((c) => c.value === currencyCode);
    if (opt) {
      this.rootField.currency_symbol!().value.set(opt.symbol);
      this.rootField.currency_locale!().value.set(opt.locale);
    }
  }

  /** AC5 — cùng quy tắc `CompanyForm.onLogoSelect` (`company-form.ts:190-215`). */
  protected onLogoSelect(event: any): void {
    const file = event.files?.[0];
    if (!file) return;

    this.logoError.set(null);

    if (file.size > CONFIG_FILE_SELECT.CONFIG_FILE_MAX_SIZE) {
      this.logoError.set(this.translate.instant('common.label.file_size_must_not_exceed_2mb'));
      event.currentFiles = [];
      return;
    }

    if (!isAllowExtensions(file.name, CONFIG_FILE_SELECT.CONFIG_FILE_ALLOW_EXT, file.type)) {
      this.logoError.set(
        this.translate.instant('common.label.allowed_formats') +
          CONFIG_FILE_SELECT.CONFIG_FILE_ALLOW_EXT.join(', '),
      );
      event.currentFiles = [];
      return;
    }

    this.setLogoFile(file);
  }

  protected onLogoRemove(): void {
    this.setLogoFile(null);
  }

  protected onLogoClear(): void {
    this.setLogoFile(null);
  }

  protected formatSize(bytes: number): string {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return Math.round((bytes / Math.pow(k, i)) * 100) / 100 + ' ' + sizes[i];
  }

  private setLogoFile(file: File | null): void {
    this.revokeLogoPreview();
    this.logoFile = file;
    this.logoError.set(null);
    this.logoPreviewUrl.set(file ? URL.createObjectURL(file) : null);
  }

  private revokeLogoPreview(): void {
    const url = this.logoPreviewUrl();
    if (url) URL.revokeObjectURL(url);
  }

  valid(): boolean {
    return !this.rootField().invalid();
  }

  markTouched(): void {
    this.rootField().markAsTouched();
  }

  value(): CompanyStepValue {
    return buildCompanyStepValue(this.model(), this.logoFile);
  }

  /**
   * AC5b (contract `OnboardingStepComponent`) — "trống" = user CHƯA NHẬP GÌ. Giá trị mặc định tự
   * điền sẵn (timezone/currency/currency_symbol/currency_locale) KHÔNG tính là "đã nhập"; logo đã
   * chọn tính là "đã nhập". Bước này `optional: false` nên `isEmpty()` không tự kích hoạt nhánh
   * auto-skip của shell — vẫn phải cài đúng theo contract (story Dev Notes).
   */
  isEmpty(): boolean {
    return computeCompanyStepIsEmpty(this.model(), this.logoFile);
  }
}
