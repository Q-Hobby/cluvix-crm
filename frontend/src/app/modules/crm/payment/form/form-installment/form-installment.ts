import { ChangeDetectionStrategy, Component, effect, inject, signal, untracked } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { disabled, FormField, minError, required, requiredError, SchemaPathTree, validate } from '@angular/forms/signals';
import { catchError, finalize, of } from 'rxjs';

import { CustomerModel } from '@/core/models/crm/customer.model';
import { PaymentModel } from '@/core/models/crm/payment.model';
import { branchActiveId, isAdmin, isDevelop } from '@/core/services/auth';
import { OrderServiceUsedService } from '@/core/services/crm/order-service-used.service';
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
import { Listbox } from 'primeng/listbox';
import { UserOptionHelperService } from '@/indexeddb/config/components/user-option';

/** ĐÚNG 16 control của `buildForm()` cũ.
 * ⚠ Bản cũ gửi payload bằng **`getRawValue()`** → **GỒM CẢ control `disabled`**, nên `amount` (bị
 * `disable()` sau khi chọn kỳ trả góp) **VẪN được gửi**. Rủi ro R1 KHÔNG áp; loại key đi mới là làm sai. */
const PAYLOAD_FIELDS = [
  'customer_id', 'branch_id', 'serviceUsed_idArr', 'installment_idArr', 'method',
  'card_type_id', 'bank_code', 'payment_time', 'payment_of_date', 'note', 'payment_for', 'created_at',
  'amount', 'updated_at',
] as const;

/** Field bind `<input pInputText>` → seed `''` (gotcha #9), kiểu `string` thuần (gotcha #28). */
const TEXT_INPUT_FIELDS = ['bank_code'] as const;

interface InstallmentFormModel {
  id?: number;
  customer_id: number | null;
  branch_id: number | null;
  /** `order_service_used_id` gửi BE */
  serviceUsed_idArr: number[];
  /** id kỳ trả góp — chỉ để hiển thị/điều khiển */
  installment_idArr: number[];
  method: number | null;
  card_type_id: number | null;
  bank_code: string;
  payment_time: Date | number | null;
  payment_of_date: Date | number | null;
  note: string | null;
  payment_for: number | null;
  created_at: number | null;
  amount: number | null;
  updated_at: Date | number | null;
}

/** Seed đủ mọi field sẽ bind, không field nào `undefined` (gotcha #1); 2 field mảng seed `[]`. */
function emptyInstallmentModel(): InstallmentFormModel {
  return {
    customer_id: null, branch_id: null, serviceUsed_idArr: [], installment_idArr: [],
    method: null, card_type_id: null, bank_code: '', payment_time: null, payment_of_date: null,
    note: null, payment_for: null, created_at: null, amount: null, updated_at: null,
  };
}

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-payment-form-installment',
  templateUrl: './form-installment.html',
  styles: [
    `
      :host ::ng-deep .p-inputgroup:has(> p-button:last-child) .p-button {
        height: 100%;
      }

      ::ng-deep .payment-listbox .p-listbox-list-wrapper {
        max-height: 290px;
      }

      ::ng-deep .payment-listbox .p-listbox-item {
        white-space: normal !important;
        padding: 10px 12px;
        line-height: 1.4;
        display: flex;
        align-items: flex-start;
        word-wrap: break-word;
        min-height: auto;
      }

      ::ng-deep .payment-listbox .p-listbox-item .p-checkbox {
        margin-right: 10px;
        margin-top: 2px;
        flex-shrink: 0;
      }

      ::ng-deep .payment-listbox .p-listbox-item-text {
        flex: 1;
        word-wrap: break-word;
        overflow-wrap: break-word;
        hyphens: auto;
      }
    `,
  ],
  standalone: true,
  imports: [
    ...FormShareImports,
    FormField,
    PaymentMethodAutocompleteFormControl,
    PaymentTypeAutocompleteFormControl,
    Listbox,
  ],
})
export class FormInstallment extends BaseSignalForm<InstallmentFormModel> {
  modelService = inject(PaymentService) as unknown as BaseApiService<InstallmentFormModel> & PaymentService;
  orderServiceUsedService = inject(OrderServiceUsedService);
  customer!: CustomerModel;
  maxAmount: number = 0;
  maxPaymentTime: Date = this.timezoneService.todayEnd();
  minPaymentTime: Date = this.timezoneService.todayStart();
  optionsTimeBefore: boolean = false;
  optionsAllowPreviousDay: boolean = false;
  readonly loadingInstallmentList = signal<boolean>(true);
  readonly installmentData = signal<
    {
      value: number; // installment_id
      order_service_used_id: number;
      label: string;
      branch_id: number;
      amount: number;
    }[]
  >([]);
  protected readonly PAYMENT_FOR = PAYMENT_FOR;
  private userOptionHelper = inject(UserOptionHelperService);
  private paymentTypeCache = inject(PaymentTypeCacheService);

  constructor() {
    super();
    this.customer = this.dialogConfig?.data?.customer;
    this.optionsTimeBefore = !!this.userOptionHelper.getUserOptionInfo(
      CONFIG_OPTION_CRM_SELECT.OPTION_TIME_BEFORE,
    )?.options; // Check option có được chọn thời gian trước đó hay không
    this.optionsAllowPreviousDay = !!this.userOptionHelper.getUserOptionInfo(
      CONFIG_OPTION_CRM_SELECT.OPTION_ALLOW_PREVIOUS_DAY,
    )?.options;
    // [QA pass 1 — finding high] Bản gốc có `method.valueChanges` → reset `card_type_id`, nằm TRONG
    // `buildForm()` nên bị xoá cùng khi convert (gotcha #45). Template form này chỉ gọi
    // `changeInstallment`, KHÔNG gọi handler nào cho `method` → phải đăng ký lại ở đây, nếu không đổi
    // phương thức thanh toán sẽ giữ nguyên loại thẻ của phương thức CŨ và lọt vào payload.
    effectOnChangeLocal(
      () => this.rootField.method?.().value() ?? null,
      () => untracked(() => this.rootField.card_type_id!().value.set(null)),
    );

  }

  override ngOnInit() {
    if (
      !this.permissionsService.getPermission(PERMISSION_SELECT.permissionUpdatePayment) &&
      !this.optionsTimeBefore
    ) {
      this.minPaymentTime = this.optionsAllowPreviousDay
        ? this.timezoneService.yesterdayStart()
        : this.timezoneService.todayStart();
    }

    this.initialValue = {
      ...this.initialValue,
      customer_id: this.customer.id,
      branch_id: branchActiveId(),
      payment_for: PAYMENT_FOR.ORDER,
    } as any;

    this._applyModel({ ...this.modelService.getInitialObject(), ...this.initialValue } as any);
    this.loadInstallmentList();
  }

  /** Merge template đủ key (gotcha #1) + 2 fallback `now()` của bản cũ. */
  private _applyModel(model: any): void {
    const merged: any = {
      ...emptyInstallmentModel(),
      ...model,
      payment_time: model.payment_time || this.today,
      payment_of_date: model.payment_of_date || this.today,
      serviceUsed_idArr: model.serviceUsed_idArr ?? [],
      installment_idArr: model.installment_idArr ?? [],
    };
    for (const key of TEXT_INPUT_FIELDS) if (merged[key] == null) merged[key] = '';
    this.model.set(merged as InstallmentFormModel);
  }

  /** Bản cũ gọi trong `buildForm()`. */
  private loadInstallmentList(): void {
    this.orderServiceUsedService
      .serviceInstallment(this.customer.id!)
      .pipe(
        takeUntilDestroyed(this.destroyRef),
        finalize(() => this.loadingInstallmentList.set(false)),
        catchError(() => of(null)),
      )
      .subscribe((res: any) => {
        if (!res) return;
        this.installmentData.set(res);
      });
  }

  /**
   * Thay đổi kỳ trả góp
   */
  changeInstallment(installmentIds: number[]) {
    if (!installmentIds?.length) {
      this.maxAmount = 0;
      this.rootField.amount!().value.set(0);
      this.rootField.serviceUsed_idArr!().value.set([]);
      return;
    }

    const total = this.calculateTotal(installmentIds);

    // Lấy danh sách order_service_used_id từ các kỳ đã chọn
    const serviceUsedIds = installmentIds
      .map((id) => this.installmentData().find((item) => item.value === id)?.order_service_used_id)
      .filter((id, index, self) => id && self.indexOf(id) === index); // Loại bỏ duplicate

    const branchId =
      installmentIds.length === 1
        ? this.installmentData().find((item) => item.value === installmentIds[0])?.branch_id ||
          branchActiveId()
        : branchActiveId();

    this.maxAmount = total;
    this.rootField.amount!().value.set(total);
    this.rootField.branch_id!().value.set(branchId ?? null);
    this.rootField.serviceUsed_idArr!().value.set(serviceUsedIds as number[]); // Gửi order_service_used_id cho BE
    // `amount` khoá/mở theo việc đã chọn kỳ trả góp hay chưa — nay khai bằng
    // `disabled(p.amount, {when})` trong `buildSchema()`; payload VẪN giữ key (bản cũ `getRawValue()`).
  }

  /**
   * Check branch user với khách hành trước khi submit lưu data
   */
  checkBeforeSubmit() {
    if (
      !isAdmin &&
      !isDevelop &&
      branchActiveId() != null &&
      branchActiveId() != this.rootField.branch_id!().value()
    ) {
      const dialogOtherBranch = this.dialogWrapperService.open(ShowOtherBranch, {
        header: this.translate.instant('common.label.notice'),
        data: {
          text: this.translate.instant('common.label.payment'),
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
    if (this.isSubmitting() || this.rootField().invalid()) return;

    this.submitted.set(true);
    this.rootField().markAsTouched();
    this.isSubmitting.set(true);

    this.modelService
      .createPaymentCustomer(this.getPayload(), {})
      .pipe(finalize(() => this.isSubmitting.set(false)), catchError(() => of(null)))
      .subscribe({
        next: (res: any) => {
          if (res) {
            this.noti.successMsg(this.translate.instant('crm.payment.message.installment_success'));
            this.dialogRef.close(res);
          }
        },
      });
  }

  /**
   * [AC15] `''` + field CHƯA dirty → `null` (giữ DB); `''` + ĐÃ dirty → `''`.
   * KHÔNG loại key theo trạng thái disable — bản cũ dùng `getRawValue()`.
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

  protected override buildSchema(p: SchemaPathTree<InstallmentFormModel>): void {
    required(p.customer_id!);
    required(p.branch_id!);
    required(p.method!);
    required(p.payment_for!);
    required(p.amount!);
    validate(p.amount!, ({ value }) => {
      const v = value();
      return v == null || Number(v) > 0 ? null : minError(0);
    });
    // 2 field MẢNG bắt buộc: `required()` KHÔNG coi `[]` là rỗng (gotcha #10) — phải viết tay, nếu không
    // sẽ submit được khi chưa chọn kỳ trả góp nào.
    validate(p.serviceUsed_idArr!, ({ value }) => ((value()?.length ?? 0) === 0 ? requiredError() : null));
    validate(p.installment_idArr!, ({ value }) => ((value()?.length ?? 0) === 0 ? requiredError() : null));
    validate(p.card_type_id!, ({ value }) => (this.cardTypeRequired() && value() == null ? requiredError() : null));
    // Bản cũ: `amount.disable()` sau khi chọn kỳ (số tiền tự tính), `enable()` khi bỏ chọn hết.
    disabled(p.amount!, { when: () => (this.rootField.installment_idArr?.().value()?.length ?? 0) > 0 });
  }

  /**
   * Tính tổng tiền các kỳ đã chọn
   */
  private calculateTotal(ids: number[]): number {
    return ids.reduce((total, id) => {
      const installment = this.installmentData().find((item) => item.value === id);
      return total + (installment?.amount || 0);
    }, 0);
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
