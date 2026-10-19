import { ChangeDetectionStrategy, Component, effect, inject, signal, untracked } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { disabled, FormField, maxError, minError, required, SchemaPathTree, validate } from '@angular/forms/signals';
import { finalize } from 'rxjs';

import { CustomerModel } from '@/core/models/crm/customer.model';
import { PaymentModel } from '@/core/models/crm/payment.model';
import { branchActiveId, isAdmin, isDepartmentBOD, isDevelop } from '@/core/services/auth';
import { PaymentService } from '@/core/services/crm/payment.service';
import { ShowOtherBranch } from '@/modules/utilities/popup-show-other-branch/show-other-branch';
import { BaseApiService } from '@/share/services/base-api.service';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import { CONFIG_OPTION_CRM_SELECT } from '@/define';
import { FormShareImports } from '@/share/import/form-share-imports';
import { UserOptionHelperService } from '@/indexeddb/config/components/user-option';

/** ĐÚNG 8 control của `buildForm()` cũ.
 *
 * ⚠ ĐÍNH CHÍNH giả định của story: form này gửi payload bằng **`getRawValue()`** ở CẢ 2 nhánh
 * (create `moveDepositToPayment`, update `updatePaymentCustomer`) — mà `getRawValue()` **GỒM CẢ control
 * `disabled`**. Nên `serviceUsed_id` (bị `disable()` khi Sửa) **VẪN được gửi** ở bản cũ.
 * ⇒ Rủi ro R1 (mất key khi disable) **KHÔNG áp** cho form này; loại key đi mới là làm sai. */
const PAYLOAD_FIELDS = [
  'serviceUsed_id', 'parent_id', 'branch_id', 'amount', 'payment_of_date', 'method', 'updated_at',
] as const;

interface MoveDepositFormModel {
  id?: number;
  serviceUsed_id: number | null;
  parent_id: number | null;
  branch_id: number | null;
  amount: number | null;
  payment_of_date: Date | number | null;
  method: number | null;
  updated_at: Date | number | null;
  /** BE trả về ở chế độ Sửa — template dùng để hiện TÊN dịch vụ (ô readonly). Chỉ ĐỌC, không nằm trong
   * `PAYLOAD_FIELDS` nên không bao giờ gửi lên. */
  serviceInfo?: { name?: string } | null;
}

/** Seed đủ mọi field sẽ bind, không field nào `undefined` (gotcha #1). Form không có field text
 * `pInputText` nào nên gotcha #9 không áp. */
function emptyMoveDepositModel(): MoveDepositFormModel {
  return {
    serviceUsed_id: null, parent_id: null, branch_id: null,
    amount: null, payment_of_date: null, method: null, updated_at: null,
  };
}

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-payment-form-move-deposit',
  templateUrl: './form-move-deposit.html',
  styles: [],
  standalone: true,
  imports: [...FormShareImports, FormField],
})
export class FormMoveDeposit extends BaseSignalForm<MoveDepositFormModel> {
  modelService = inject(PaymentService) as unknown as BaseApiService<MoveDepositFormModel> & PaymentService;
  /** `BaseForm` có sẵn `canEditBranch`; `BaseSignalForm` thì KHÔNG. Form này chưa bao giờ set `false`. */
  protected readonly canEditBranch = signal(true);
  optionsTimeBefore: boolean;
  optionsAllowPreviousDay: boolean = false;
  maxDate: Date = this.timezoneService.todayEnd();
  minDate: Date = this.timezoneService.todayStart();
  customer!: CustomerModel;
  idDeposit!: number; // id đặt cọc
  max: number = 0; // số tiền cọc lớn nhất
  readonly serviceUsed = signal<
    { value: number; label: string; number: number; parent_id: number; created_day: number }[]
  >([]);
  private userOptionHelper = inject(UserOptionHelperService);

  constructor() {
    super();

    this.customer = this.dialogConfig?.data?.customer;
    this.idDeposit = this.dialogConfig?.data?.idDeposit;
    this.max = this.dialogConfig?.data?.depositAfter;
    this.serviceUsed.set(this.dialogConfig?.data?.serviceUsed ?? []);

    // Check option có được chọn thời gian trước đó hay không
    this.optionsTimeBefore = !!this.userOptionHelper.getUserOptionInfo(
      CONFIG_OPTION_CRM_SELECT.OPTION_TIME_BEFORE,
    )?.options;
    this.optionsAllowPreviousDay = !!this.userOptionHelper.getUserOptionInfo(
      CONFIG_OPTION_CRM_SELECT.OPTION_ALLOW_PREVIOUS_DAY,
    )?.options;

    // `normalizeDateField()` là helper CHỈ có ở `BaseForm` (không có ở `BaseApp`/`BaseSignalForm`) — viết
    // lại tại chỗ cho đúng 1 field mà form này cần. Chạy 1 lần sau khi model đã seed.
    queueMicrotask(() => this.normalizePaymentOfDate());

    // Cascade: đổi dịch vụ → set `parent_id` theo dịch vụ, và tính lại `minDate`.
    // (bản cũ: `valueChanges` của `serviceUsed_id` đăng ký trong `buildForm()`).
    effectOnChangeLocal(
      () => this.rootField.serviceUsed_id?.().value() ?? null,
      id => {
        const found = this.serviceUsed()?.find?.(s => s.value === Number(id));
        if (!found) return;
        untracked(() => {
          if (found.parent_id !== undefined) this.rootField.parent_id!().value.set(found.parent_id);
        });
        if (!this.optionsTimeBefore && !isDepartmentBOD()) {
          this.minDate = this.optionsAllowPreviousDay
            ? this.timezoneService.yesterdayStart()
            : this.timezoneService.todayStart();
        } else {
          this.minDate = new Date(found.created_day * 1000); // Min là ngày tạo dịch vụ
        }
      },
    );
  }

  /** Bản rút gọn của `BaseForm.normalizeDateField()` cho đúng field `payment_of_date`. */
  private normalizePaymentOfDate(): void {
    const field = this.rootField.payment_of_date;
    if (!field) return;
    const val = field().value();
    if (!val) {
      field().value.set(this.timezoneService.now());
    } else if (typeof val === 'number') {
      field().value.set(new Date(val < 1e12 ? val * 1000 : val));
    } else if (typeof val === 'string') {
      field().value.set(new Date(val));
    }
  }

  override ngOnInit() {
    const dialogModel = this.model() as any;
    if (dialogModel && Object.keys(dialogModel).length > 0) {
      // `serviceUsed_id` khoá khi Sửa → nay khai bằng `disabled(..., {when: isEditMode})` trong
      // `buildSchema()`; payload VẪN giữ key (bản cũ dùng `getRawValue()`).
      this._applyModel(dialogModel);

      const depositAfter: any = dialogModel.deposit?.deposit_after;
      if (+depositAfter > 0) {
        this.max = +(dialogModel.amount ?? 0) + +depositAfter; // số tiền cọc còn lại
      } else {
        this.max = dialogModel.amount ?? 0;
      }
      if (isDepartmentBOD()) {
        this.minDate = new Date((dialogModel.orderServiceUsed?.created_day ?? 0) * 1000);
      }
    } else {
      this.initialValue = {
        ...this.initialValue,
        customer_id: this.customer?.id,
        branch_id: branchActiveId(),
        parent_id: this.dialogConfig?.data?.parent_id || null,
        payment_of_date: this.today,
      } as any;
      this._applyModel({ ...this.modelService.getInitialObject(), ...this.initialValue } as any);
    }
  }

  /** Merge template đủ key (gotcha #1). */
  private _applyModel(model: any): void {
    this.model.set({ ...emptyMoveDepositModel(), ...model } as MoveDepositFormModel);
  }

  /**
   * Hàm xử lý chuyển max tiền cọc hoặc max tiền dịch vụ nếu tiền cọc lớn hơn tiền dịch vụ
   */
  maxDeposit() {
    const id = this.rootField.serviceUsed_id!().value();
    const found = this.serviceUsed().find((item) => item.value === id);
    if (!found) {
      console.warn(this.translate.instant('crm.payment.message.service_not_found_for_max'));
      return;
    }
    const amountF = Math.min(Number(this.max), Number(found.number));
    this.rootField.amount!().value.set(amountF);
  }

  /**
   * Trần số tiền chuyển: min(số dư cọc, nợ còn lại của dịch vụ). Chế độ Sửa không có danh sách nợ dịch vụ
   * (dialog không nạp) ⇒ chỉ giới hạn theo cọc; BE kiểm nợ dịch vụ trong transaction.
   */
  private amountLimit(serviceUsedId: number | null): number | null {
    const limits: number[] = [];
    if (Number(this.max) > 0) limits.push(Number(this.max));
    const found = this.serviceUsed().find((item) => item.value === Number(serviceUsedId));
    if (found) limits.push(Number(found.number));
    return limits.length ? Math.min(...limits) : null;
  }

  /** Trần đang vi phạm (đã định dạng) để hiện dưới ô số tiền; null khi không vượt. */
  protected amountMaxHint(): string | null {
    const err = this.rootField.amount!().errors().find((e) => e.kind === 'max') as { max?: number } | undefined;
    return err?.max != null ? Number(err.max).toLocaleString('vi-VN') : null;
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
          text: this.translate.instant('common.label.transfer_deposit'),
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

  override onSubmit(): void {
    if (this.isSubmitting()) return;
    this.submitted.set(true);
    this.rootField().markAsTouched();

    if (this.rootField().invalid()) return;

    this.isSubmitting.set(true);

    if (this.isEditMode()) {
      this.modelService
        .updatePaymentCustomer(this.model().id!, this.getPayload(), {})
        .pipe(finalize(() => this.isSubmitting.set(false)))
        .subscribe({
          next: (res: any) => {
            this.noti.successMsg(this.translate.instant('crm.payment.message.update_transfer_deposit_success'));
            this.dialogRef.close(res);
          },
        });
    } else {
      this.modelService
        .moveDepositToPayment(this.idDeposit, this.getPayload(), {})
        .pipe(finalize(() => this.isSubmitting.set(false)))
        .subscribe({
          next: (res: any) => {
            this.noti.successMsg(this.translate.instant('crm.payment.message.transfer_deposit_success'));
            this.dialogRef.close(res);
          },
        });
    }
  }

  /**
   * Khởi tạo form
   * @param model
   */
  protected override getPayload(): any {
    const raw = super.getPayload() as any;
    const payload: any = {};
    // KHÔNG loại key nào theo trạng thái disable — bản cũ dùng `getRawValue()` (xem ghi chú
    // ở `PAYLOAD_FIELDS`).
    for (const key of PAYLOAD_FIELDS) payload[key] = raw[key];
    return payload;
  }

  protected override buildSchema(p: SchemaPathTree<MoveDepositFormModel>): void {
    required(p.serviceUsed_id!);
    required(p.parent_id!);
    required(p.branch_id!);
    required(p.amount!);
    // `greaterThanNumber(0)` — KHÔNG `min()` (gotcha #20).
    validate(p.amount!, ({ value }) => {
      const v = value();
      return v == null || Number(v) > 0 ? null : minError(0);
    });
    // lens-money M-4: số chuyển không vượt số dư cọc VÀ không vượt nợ còn lại của dịch vụ đã chọn
    // (BE chặn lại trong transaction — đây chỉ để báo sớm, ô nhập trước đây gõ được số lớn hơn).
    validate(p.amount!, ({ value, valueOf }) => {
      const v = value();
      if (v == null) return null;
      const limit = this.amountLimit(valueOf(p.serviceUsed_id!));
      return limit != null && Number(v) > limit ? maxError(limit) : null;
    });
    // Bản cũ: `rfModel.get('serviceUsed_id')?.disable()` khi Sửa.
    disabled(p.serviceUsed_id!, { when: () => this.isEditMode() });
  }
}

/** Copy helper `effectOnChange` (quyết định epic: KHÔNG import chung) — bỏ qua lần chạy đầu để cascade
 * không đè dữ liệu vừa nạp ở chế độ Sửa. */
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
