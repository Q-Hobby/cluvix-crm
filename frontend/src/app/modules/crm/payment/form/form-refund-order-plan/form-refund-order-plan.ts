import { ChangeDetectionStrategy, Component, effect, inject, untracked } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormField, minError, required, requiredError, SchemaPathTree, validate } from '@angular/forms/signals';
import { catchError, finalize, of } from 'rxjs';

import { CustomerModel } from '@/core/models/crm/customer.model';
import { PaymentModel } from '@/core/models/crm/payment.model';
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

/** ĐÚNG 13 control của `buildForm()` cũ. */
const PAYLOAD_FIELDS = [
  'customer_id', 'branch_id', 'parent_id', 'serviceUsed_id', 'method', 'card_type_id',
  'bank_code', 'payment_time', 'note', 'payment_for', 'amount', 'updated_at',
] as const;

/** Field bind `<input pInputText>` → seed `''` (gotcha #9), kiểu `string` thuần (gotcha #28). */
const TEXT_INPUT_FIELDS = ['bank_code'] as const;

interface RefundOrderPlanFormModel {
  id?: number;
  customer_id: number | null;
  branch_id: number | null;
  parent_id: number | null;
  serviceUsed_id: number | null;
  method: number | null;
  card_type_id: number | null;
  bank_code: string;
  payment_time: Date | number | null;
  note: string | null;
  payment_for: number | null;
  amount: number | null;
  updated_at: Date | number | null;
}

/** Seed đủ mọi field sẽ bind, không field nào `undefined` (gotcha #1). */
function emptyRefundOrderPlanModel(): RefundOrderPlanFormModel {
  return {
    customer_id: null, branch_id: null, parent_id: null, serviceUsed_id: null,
    method: null, card_type_id: null, bank_code: '', payment_time: null, note: null,
    payment_for: PAYMENT_FOR.SERVICE_REFUND, amount: null, updated_at: null,
  };
}

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-payment-form-refund-deposit-order-plan',
  templateUrl: './form-refund-order-plan.html',
  standalone: true,
  imports: [
    ...FormShareImports,
    FormField,
    PaymentMethodAutocompleteFormControl,
    PaymentTypeAutocompleteFormControl,
  ],
})
export class FormRefundOrderPlan extends BaseSignalForm<RefundOrderPlanFormModel> {
  modelService = inject(PaymentService) as unknown as BaseApiService<RefundOrderPlanFormModel> & PaymentService;
  idOrderPlan!: number; // id của đơn hàng
  idService!: number; // id của dịch vụ
  customer!: CustomerModel; // khách hàng
  maxPaymentTime: Date = this.timezoneService.todayEnd(); // lưu thời gian thanh toán lớn nhất
  minPaymentTime!: Date; // lưu thời gian thanh toán nhỏ nhất
  optionsTimeBefore: boolean = false; // biến lưu option thời gian trước đó hay không
  optionsAllowPreviousDay: boolean = false;
  payment: number;
  private userOptionHelper = inject(UserOptionHelperService);
  private paymentTypeCache = inject(PaymentTypeCacheService);

  constructor() {
    super();
    this.customer = this.dialogConfig?.data?.customer;
    this.idOrderPlan = this.dialogConfig?.data?.idOrderPlan; // lấy id của đơn hàng từ ngoài truyền vào
    this.idService = this.dialogConfig?.data?.idService; // lấy id của dịch vụ từ ngoài truyền vào
    this.optionsTimeBefore = !!this.userOptionHelper.getUserOptionInfo(
      CONFIG_OPTION_CRM_SELECT.OPTION_TIME_BEFORE,
    )?.options; // Check option có được chọn thời gian trước đó hay không
    this.optionsAllowPreviousDay = !!this.userOptionHelper.getUserOptionInfo(
      CONFIG_OPTION_CRM_SELECT.OPTION_ALLOW_PREVIOUS_DAY,
    )?.options;
    this.payment = this.dialogConfig?.data?.payment;
    // Bản cũ: `method.valueChanges` → reset `card_type_id` (phần đổi validator nay là điều kiện tĩnh
    // `cardTypeRequired()` trong `buildSchema()`). Template form này KHÔNG gọi handler nào nên cascade
    // phải đăng ký ở đây, nếu không đổi phương thức sẽ giữ nguyên loại thẻ cũ.
    effectOnChangeLocal(
      () => this.rootField.method?.().value() ?? null,
      () => untracked(() => this.rootField.card_type_id!().value.set(null)),
    );

  }

  /**
   * Form CHỈ tạo phiếu hoàn dịch vụ (O-4, 2026-10-18): BE không có route sửa phiếu hoàn dịch vụ
   * (`crm/payment` chỉ có create/list/delete-refund-order-plan) ⇒ bỏ chế độ sửa (từng gọi
   * `PUT …/update-refund-order-plan` → 404). Sai phiếu thì xoá rồi tạo lại.
   */
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
      customer_id: this.customer.id,
      branch_id: this.customer.branch_id,
      payment_time: this.today,
    } as any;

    this._applyModel({ ...this.modelService.getInitialObject(), ...this.initialValue } as any);
  }

  /** Merge template đủ key (gotcha #1); `parent_id`/`serviceUsed_id` lấy từ dialog, `payment_for` là hằng. */
  private _applyModel(model: any): void {
    const merged: any = {
      ...emptyRefundOrderPlanModel(),
      ...model,
      parent_id: this.idOrderPlan ?? null,
      serviceUsed_id: this.idService ?? null,
      payment_for: PAYMENT_FOR.SERVICE_REFUND,
    };
    for (const key of TEXT_INPUT_FIELDS) if (merged[key] == null) merged[key] = '';
    this.model.set(merged as RefundOrderPlanFormModel);
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
          text: this.translate.instant('common.label.refund_service'),
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

  override onSubmit() {
    if (this.isSubmitting()) return;
    this.submitted.set(true);
    this.rootField().markAsTouched();

    if (this.rootField().invalid()) return;

    this.isSubmitting.set(true);

    this.modelService
      .createRefundOrderPlan(this.getPayload(), {})
      .pipe(finalize(() => this.isSubmitting.set(false)), catchError(() => of(null)))
      .subscribe({
        next: (res: any) => {
          if (res) {
            this.noti.successMsg(this.translate.instant('common.message.create_success'));
            this.dialogRef.close(res);
          }
        },
      });
  }

  /**
   * [AC15] `''` + field CHƯA dirty → `null` (giữ DB); `''` + ĐÃ dirty → `''` (user cố ý xoá).
   * Chi tiết: `form-deposit.ts` (pilot nhóm A).
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

  protected override buildSchema(p: SchemaPathTree<RefundOrderPlanFormModel>): void {
    required(p.customer_id!);
    required(p.branch_id!);
    required(p.method!);
    required(p.amount!);
    validate(p.amount!, ({ value }) => {
      const v = value();
      return v == null || Number(v) > 0 ? null : minError(0);
    });
    validate(p.card_type_id!, ({ value }) => (this.cardTypeRequired() && value() == null ? requiredError() : null));
  }
}


/** Copy helper `effectOnChange` (quyết định epic: KHÔNG import chung) — bỏ qua lần chạy đầu để không
 * xoá `card_type_id` vừa nạp ở chế độ Sửa. */
function effectOnChangeLocal<V>(read: () => V, action: (value: V) => void): void {
  let initialized = false;
  let prev: V;
  effect(() => {
    const value = read();
    if (!initialized) { initialized = true; prev = value; return; }
    if (value === prev) return;
    prev = value;
    action(value);
  });
}
