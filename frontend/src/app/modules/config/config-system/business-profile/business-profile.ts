import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { FormField, SchemaPathTree } from '@angular/forms/signals';
import { finalize } from 'rxjs';

import { BusinessProfileModel } from '@/core/models/config/system/business-profile.model';
import { authBusinessProfile } from '@/core/services/auth/auth.state';
import { BusinessProfileService } from '@/core/services/config/system/business-profile.service';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import { FormShareImports } from '@/share/import/form-share-imports';
import {
  CONFIG_FILE_SELECT,
  CURRENCY_OPTIONS,
  DEFAULT_CURRENCY,
  DEFAULT_CURRENCY_LOCALE,
  DEFAULT_CURRENCY_SYMBOL,
  DEFAULT_TIMEZONE,
  TIMEZONE_OPTIONS,
  WEEKDAY_OPTIONS,
} from '@/define';
import { isAllowExtensions } from '@/share/services/utils.service';
import { translateOptions } from '@/share/utils/translate-options.util';

/**
 * Hồ sơ doanh nghiệp — thay màn "Thương hiệu" (`config/company`, story-42/61 đã bỏ chọn thương
 * hiệu; story-43 bỏ hẳn khái niệm company). 1 dòng DUY NHẤT (`GET/PUT /api/config/business-profile`,
 * BE story-63) — KHÔNG list, KHÔNG id trên path, luôn "edit mode" (upsert phía BE).
 *
 * Signal Forms cần KEY tồn tại thật + giá trị KHÔNG `undefined` (gotcha #1 angular22.md).
 */
function emptyBusinessProfileModel(): BusinessProfileModel {
  return {
    name: '', code: '', hotline: '', address: '', email: '', website: '',
    timezone: DEFAULT_TIMEZONE, currency: DEFAULT_CURRENCY,
    currency_symbol: DEFAULT_CURRENCY_SYMBOL, currency_locale: DEFAULT_CURRENCY_LOCALE,
    weekly_rest_days: [], allowed_ips: '', require_2fa: 0, require_2fa_roles: [],
    totp_trust_days: 30, tax_code: '', logo_image: null, updated_at: 0,
  } as unknown as BusinessProfileModel;
}

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-business-profile',
  templateUrl: './business-profile.html',
  styles: [''],
  imports: [...FormShareImports, FormField],
  standalone: true,
})
export class BusinessProfile extends BaseSignalForm<BusinessProfileModel> {
  modelService = inject(BusinessProfileService);

  logoFile: File | null = null;
  private logoCleared = false;
  srcLogoUpload: any;
  logoError = signal<string | null>(null);
  logoLoading = signal(false);

  protected readonly blockSpace: RegExp = /[^\s]/;
  protected readonly CONFIG_FILE_SELECT = CONFIG_FILE_SELECT;
  protected readonly TIMEZONE_OPTIONS = TIMEZONE_OPTIONS;
  protected readonly CURRENCY_OPTIONS = CURRENCY_OPTIONS;
  protected readonly WEEKDAY_OPTIONS = translateOptions(WEEKDAY_OPTIONS);

  roleOptions = signal<any[]>([]);

  override ngOnInit() {
    this.rbacService.getChildRoleForDropdown().subscribe({
      next: (res: any) => this.roleOptions.set(Array.isArray(res) ? res : res?.data || []),
      error: () => this.roleOptions.set([]),
    });

    this.model.set({ ...emptyBusinessProfileModel() });
    this.modelService.getProfile().subscribe((loaded) => this._applyModel(loaded));
  }

  /** Chuẩn hoá model trước khi set vào FieldTree — convert CSV/JSON (BE) <-> mảng/text (UI). */
  private _applyModel(model: any): void {
    const normalized: Partial<BusinessProfileModel> = {
      ...model,
      weekly_rest_days: this._csvToWeekdays(model?.weekly_rest_days),
      allowed_ips: this._ipsToText(model?.allowed_ips),
      require_2fa_roles: this._parseRoles(model?.require_2fa_roles),
      require_2fa: model?.require_2fa === 1 || model?.require_2fa === true ? 1 : 0,
      totp_trust_days: model?.totp_trust_days ?? 30,
    };

    if (model?.logo_image) {
      this.logoLoading.set(true);
      this.srcLogoUpload = model.logo_image;
    }

    this.model.set({ ...emptyBusinessProfileModel(), ...normalized } as BusinessProfileModel);
  }

  protected override buildSchema(_p: SchemaPathTree<BusinessProfileModel>): void {
    // Không field nào bắt buộc cứng — hồ sơ có thể để trống lúc mới cài đặt.
  }

  override onSubmit(): void {
    if (this.isSubmitting()) return;

    this.submitted.set(true);
    this.rootField().markAsTouched();
    if (this.rootField().invalid()) return;

    const formValue: any = { ...this.getPayload() };
    formValue.allowed_ips = this._parseIps(formValue.allowed_ips);
    formValue.weekly_rest_days = this._weekdaysToCsv(formValue.weekly_rest_days);
    formValue.require_2fa_roles = formValue.require_2fa_roles ?? [];
    formValue.logoFile = this.logoFile;

    const hasFile = !!this.logoFile || this.logoCleared;

    this.isSubmitting.set(true);

    this.modelService
      .updateProfile(formValue, hasFile)
      .pipe(finalize(() => this.isSubmitting.set(false)))
      .subscribe({
        next: (loaded) => {
          this.noti.successMsg(this.translate.instant('config.business_profile.update_success'));
          // Signal toàn app (timezone, tiền tệ, logo/tên topbar + bản in) đọc ngay bản vừa lưu — không
          // set thì phải tải lại trang mới thấy thay đổi.
          authBusinessProfile.set(loaded);
          this._applyModel(loaded);
        },
      });
  }

  onLogoSelect(event: any) {
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
        this.translate.instant('common.label.allowed_formats') + CONFIG_FILE_SELECT.CONFIG_FILE_ALLOW_EXT.join(', '),
      );
      event.currentFiles = [];
      return;
    }
    this.logoFile = file;
  }

  onLogoRemove() {
    this.logoFile = null;
    this.logoError.set(null);
  }

  onLogoClear() {
    this.logoFile = null;
    this.logoError.set(null);
  }

  clearExistingLogo() {
    this.srcLogoUpload = null;
    this.logoFile = null;
    this.logoCleared = true;
    this.logoError.set(null);
    this.rootField.logo_image!().value.set(null);
  }

  formatSize(bytes: number): string {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return Math.round((bytes / Math.pow(k, i)) * 100) / 100 + ' ' + sizes[i];
  }

  onLogoLoad() {
    setTimeout(() => this.logoLoading.set(false), 500);
  }

  onCurrencyChange(currencyCode: string) {
    const opt = CURRENCY_OPTIONS.find((c) => c.value === currencyCode);
    if (opt) {
      this.rootField.currency_symbol!().value.set(opt.symbol);
      this.rootField.currency_locale!().value.set(opt.locale);
    }
  }

  private _ipsToText(v: any): string {
    if (Array.isArray(v)) return v.join('\n');
    if (typeof v === 'string' && v) {
      try {
        const a = JSON.parse(v);
        return Array.isArray(a) ? a.join('\n') : v;
      } catch {
        return v;
      }
    }
    return '';
  }

  private _parseIps(text: any): string[] {
    if (Array.isArray(text)) return text;
    if (typeof text !== 'string') return [];
    return text
      .split(/[\n,]/)
      .map((s) => s.trim())
      .filter(Boolean);
  }

  private _csvToWeekdays(v: any): number[] {
    if (Array.isArray(v)) return v.map((x) => Number(x)).filter((n) => !isNaN(n));
    if (typeof v !== 'string' || !v.trim()) return [];
    return v
      .split(',')
      .map((s) => Number(s.trim()))
      .filter((n) => !isNaN(n));
  }

  private _weekdaysToCsv(v: any): string {
    if (!Array.isArray(v)) return '';
    return v.map((n) => Number(n)).filter((n) => !isNaN(n)).join(',');
  }

  private _parseRoles(v: any): string[] {
    if (Array.isArray(v)) return v;
    if (typeof v === 'string' && v) {
      try {
        const a = JSON.parse(v);
        return Array.isArray(a) ? a : [];
      } catch {
        return [];
      }
    }
    return [];
  }
}
