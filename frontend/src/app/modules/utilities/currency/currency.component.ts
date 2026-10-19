import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { FormField, required, validate, minError, SchemaPathTree } from '@angular/forms/signals';
import { finalize } from 'rxjs';

import { CurrencyService } from '@/modules/utilities/currency/currency.service';
import { LocaleFormatService } from '@/share/services/locale-format.service';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import { CURRENCY_OPTION } from '@/define';
import { FormShareImports } from '@/share/import/form-share-imports';

/**
 * Model cục bộ — KHÔNG dùng `BaseSignalForm<any>`: `FieldTree<any>` dính quirk conditional type
 * distribute trên `any`, làm hỏng cả truy cập property lẫn tham số hàm (gotcha #4 `signal-forms.md`).
 * `id` cố ý KHÔNG có key: form này không CRUD, chỉ gọi API convert.
 */
interface CurrencyConvertModel {
  id?: number;
  from: string | null;
  to: string | null;
  num: number | null;
}

@Component({
  selector: 'app-currency',
  templateUrl: './currency.component.html',
  styles: [],
  imports: [...FormShareImports, FormField],
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class CurrencyComponent extends BaseSignalForm<CurrencyConvertModel> {
  protected readonly currencyConvert = signal<any>(null);
  modelService = inject(CurrencyService);
  private localeFormat = inject(LocaleFormatService);
  protected readonly CURRENCY_OPTION = CURRENCY_OPTION;

  /**
   * Seed giá trị mặc định giống hệt `buildForm()` cũ. KHÔNG gọi `super.ngOnInit()` — `CurrencyService`
   * không override `getInitialObject()` (trả `{}` mặc định của `BaseApiService`), seed qua đường đó sẽ
   * để field `undefined` → không bind được vĩnh viễn (gotcha #1).
   */
  override ngOnInit(): void {
    this.model.set({
      from: 'USD',
      num: 1,
      to: this.localeFormat.currencyCode(),
    });
  }

  override onSubmit() {
    if (this.isSubmitting()) return;
    this.submitted.set(true);
    this.rootField().markAsTouched();

    if (this.rootField().invalid()) return;

    const fromValue = this.rootField.from!().value();
    const toValue = this.rootField.to!().value();

    // Validation: không cho convert cùng loại tiền
    if (fromValue === toValue) {
      this.noti.warnMsg(this.translate.instant('currency.same_unit_error'));
      return;
    }

    this.isSubmitting.set(true);
    this.loadingService.showLoading('convertCurrency');

    this.modelService
      .convertCurrency(this.getPayload())
      .pipe(
        finalize(() => {
          this.isSubmitting.set(false);
          this.loadingService.hideLoading('convertCurrency');
        }),
      )
      .subscribe({
        next: (res: any) => {
          this.noti.successMsg(this.translate.instant('utilities.currency.convert_success'));
          this.currencyConvert.set(res);
        },
      });
  }

  cancelSave(): void {
    this.dialogRef.close(false);
  }

  protected override buildSchema(p: SchemaPathTree<CurrencyConvertModel>): void {
    required(p.from!);
    required(p.to!);
    required(p.num!);
    // `min(p.num, 0)` sẽ đẩy metadata xuống `@Input min` của `p-inputnumber` làm ô số CLAMP giá trị
    // thay vì báo đỏ như bản cũ (gotcha #20) — dùng `validate()` để giữ nguyên UX.
    validate(p.num!, ({ value }) => {
      const v = value();
      return v != null && v < 0 ? minError(0) : null;
    });
  }
}
