import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { catchError, finalize, of } from 'rxjs';
import { FormField, minError, required, requiredError, SchemaPathTree, validate } from '@angular/forms/signals';

import { CustomerModel } from '@/core/models/crm/customer.model';
import { branchActiveId, isAdmin, isDevelop } from '@/core/services/auth';
import { PaymentService } from '@/core/services/crm/payment.service';
import { ShowOtherBranch } from '@/modules/utilities/popup-show-other-branch/show-other-branch';
import { BaseApiService } from '@/share/services/base-api.service';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import {
  CONFIG_OPTION_CRM_SELECT,

  PAYMENT_FOR,
  PERMISSION_SELECT,
} from '@/define';
import { FormShareImports } from '@/share/import/form-share-imports';
import { PaymentMethodAutocompleteFormControl } from '@/indexeddb/config/components/payment-method';
import {
  PaymentTypeAutocompleteFormControl,
  PaymentTypeCacheService,
} from '@/indexeddb/config/components/payment-type';
import { UserOptionHelperService } from '@/indexeddb/config/components/user-option';

/** ĐÚNG 14 control của `buildForm()` cũ. */
const PAYLOAD_FIELDS = [
  'customer_id', 'branch_id', 'deposit_id', 'method', 'card_type_id', 'bank_code',
  'payment_time', 'note', 'payment_for', 'amount', 'created_at', 'updated_at',
] as const;

/** Field bind `<input pInputText type="text">` → seed `''` (gotcha #9), kiểu `string` thuần (gotcha #28). */
const TEXT_INPUT_FIELDS = ['bank_code'] as const;

interface RefundKgFormModel {
  id?: number;
  customer_id: number | null;
  branch_id: number | null;
  deposit_id: number | null;
  method: number | null;
  card_type_id: number | null;
  bank_code: string;
  payment_time: Date | number | null;
  note: string | null;
  payment_for: number | null;
  amount: number | null;
  created_at: number | null;
  updated_at: Date | number | null;
}

/** Seed đủ mọi field sẽ bind, không field nào `undefined` (gotcha #1). */
function emptyRefundKgModel(): RefundKgFormModel {
  return {
    customer_id: null, branch_id: null, deposit_id: null, method: null,
    card_type_id: null, bank_code: '', payment_time: null, note: null, payment_for: PAYMENT_FOR.CONSIGNMENT_REFUND,
    amount: null, created_at: null, updated_at: null,
  };
}

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-payment-form-refund-deposit-kg',
  templateUrl: './form-refund-kg.html',
  standalone: true,
  imports: [
    ...FormShareImports,
    FormField,
    PaymentMethodAutocompleteFormControl,
    PaymentTypeAutocompleteFormControl,
  ],
})
export class FormRefundKg extends BaseSignalForm<RefundKgFormModel> {
  modelService = inject(PaymentService) as unknown as BaseApiService<RefundKgFormModel> & PaymentService;
  /** `BaseForm` có sẵn `canEditBranch = signal(true)`; `BaseSignalForm` thì KHÔNG. Form này chưa bao giờ
   * set `false` → khai lại mặc định `true`, giữ đúng hành vi cũ (tiền lệ `promotion-form.ts`). */
  protected readonly canEditBranch = signal(true);
  idDepositPayment!: number;
  customer!: CustomerModel;
  maxPaymentTime: Date = this.timezoneService.todayEnd();
  minPaymentTime!: Date;
  optionsTimeBefore: boolean = false; // biến lưu option thời gian trước đó hay không
  optionsAllowPreviousDay: boolean = false;
  private userOptionHelper = inject(UserOptionHelperService);
  private paymentTypeCache = inject(PaymentTypeCacheService);

  constructor() {
    super();
    this.customer = this.dialogConfig?.data?.customer;
    this.idDepositPayment = this.dialogConfig?.data?.idDeposit;
    this.optionsTimeBefore = !!this.userOptionHelper.getUserOptionInfo(
      CONFIG_OPTION_CRM_SELECT.OPTION_TIME_BEFORE,
    )?.options; // Check option có được chọn thời gian trước đó hay không
    this.optionsAllowPreviousDay = !!this.userOptionHelper.getUserOptionInfo(
      CONFIG_OPTION_CRM_SELECT.OPTION_ALLOW_PREVIOUS_DAY,
    )?.options;
  }

  override ngOnInit() {
    if (!this.permissionsService.getPermission(PERMISSION_SELECT.permissionUpdatePayment)) {
      if (!this.optionsTimeBefore) {
        this.minPaymentTime = this.optionsAllowPreviousDay
          ? this.timezoneService.yesterdayStart()
          : this.timezoneService.todayStart();
      }
    }
    this.initialValue = {
      ...this.initialValue,
      customer_id: this.customer?.id,
      branch_id: branchActiveId(),
      payment_time: this.today,
    } as any;

    this._applyModel({ ...this.modelService.getInitialObject(), ...this.initialValue } as any);
  }

  /**
   * Check branch nhân viên với khách hành trước khi submit lưu data
   */
  checkBeforeSubmit() {
    // nếu như branch nhân viên khác với branch khách hàng thì show popup cảnh báo, trừ admin và develop
    if (
      !isAdmin &&
      !isDevelop &&
      branchActiveId() != null &&
      branchActiveId() != this.rootField.branch_id!().value()
    ) {
      const dialogOtherBranch = this.dialogWrapperService.open(ShowOtherBranch, {
        header: this.translate.instant('common.label.notice'),
        data: {
          text: this.translate.instant('crm.payment.message.text_refund_consignment'),
        },
        width: '30%',
      });

      dialogOtherBranch.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((result: any) => {
        if (result) {
          this.onSubmit();
        }
      });
    } else {
      this.onSubmit();
    }
  }

  /** Bản cũ vừa reset `card_type_id` vừa đổi validator; phần validator nay là điều kiện tĩnh trong
   * `buildSchema()` (`cardTypeRequired()`), hàm này chỉ còn phần reset. */
  changeMethod(_methodId: number | null) {
    this.rootField.card_type_id!().value.set(null);
  }

  override onSubmit() {
    if (this.isSubmitting()) return;
    this.submitted.set(true);
    this.rootField().markAsTouched();

    if (this.rootField().invalid()) return;

    this.isSubmitting.set(true);
    this.modelService
      .createRefundKg(this.getPayload(), {})
      .pipe(
        finalize(() => {
          this.isSubmitting.set(false);
        }),
        catchError(() => of(null)),
      )
      .subscribe({
        next: (res: any) => {
          if (res) {
            this.noti.successMsg(this.translate.instant('crm.payment.message.refund_consignment_success'));
            this.dialogRef.close(res);
          }
        },
      });
  }

  /**
   * Khởi tạo form
   * @param model
   */
  /** Merge template đủ key (gotcha #1); `payment_for` là hằng cứng, `deposit_id` lấy từ dialog. */
  private _applyModel(model: any): void {
    const merged: any = {
      ...emptyRefundKgModel(),
      ...model,
      deposit_id: this.idDepositPayment ?? null,
      payment_for: PAYMENT_FOR.CONSIGNMENT_REFUND,
    };
    for (const key of TEXT_INPUT_FIELDS) {
      if (merged[key] == null) merged[key] = '';
    }
    this.model.set(merged as RefundKgFormModel);
  }

  /**
   * [AC15] `''` + field CHƯA dirty → `null` (chưa chạm, giữ DB); `''` + ĐÃ dirty → `''` (user cố ý xoá).
   * BE `ToParams()` bỏ qua con trỏ `nil` nên `null` ≡ vắng key ≡ giữ nguyên DB.
   * Chi tiết đầy đủ: `form-deposit.ts` (pilot nhóm A).
   */
  protected override getPayload(): any {
    const raw = super.getPayload() as any;
    const payload: any = {};
    for (const key of PAYLOAD_FIELDS) payload[key] = raw[key];
    for (const key of TEXT_INPUT_FIELDS) {
      if (payload[key] === '' && !this.rootField[key]?.().dirty()) payload[key] = null;
    }
    return payload;
  }

  /** `card_type_id` bắt buộc khi phương thức đang chọn CÓ loại thẻ trong cache. */
  private cardTypeRequired(): boolean {
    const methodId = this.rootField.method?.().value();
    if (!methodId) return false;
    return this.paymentTypeCache.getByPaymentMethodId(methodId).length > 0;
  }

  protected override buildSchema(p: SchemaPathTree<RefundKgFormModel>): void {
    required(p.customer_id!);
    required(p.branch_id!);
    required(p.method!);
    required(p.amount!);
    // `greaterThanNumber(0)` — KHÔNG dùng `min()` (đẩy metadata → ô số CLAMP, gotcha #20).
    validate(p.amount!, ({ value }) => {
      const v = value();
      return v == null || Number(v) > 0 ? null : minError(0);
    });
    validate(p.card_type_id!, ({ value }) => (this.cardTypeRequired() && value() == null ? requiredError() : null));
  }
}
