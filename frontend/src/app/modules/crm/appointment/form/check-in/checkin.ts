import { ChangeDetectionStrategy, Component, computed, effect, inject, signal, untracked } from '@angular/core';
import { finalize } from 'rxjs';
import { disabled, FormField, requiredError, SchemaPathTree, validate } from '@angular/forms/signals';

import { AppointmentModel } from '@/core/models/crm/appointment.model';
import { AppointmentService } from '@/core/services/crm/appointment.service';
import { BaseApiService } from '@/share/services/base-api.service';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import {
  APPOINTMENT_IS_ACCEPTED_SELECT,
  APPOINTMENT_TYPE_SELECT,
} from '@/define';
import { FormShareImports } from '@/share/import/form-share-imports';
import { AppointmentReasonAutocompleteFormControl } from '@/indexeddb/config/components/appointment-reason';

/** Copy nguyên helper từ `service-form.ts` (quyết định đã chốt của epic: KHÔNG import chung) — bỏ qua
 * lần chạy ĐẦU để cascade reset không đè dữ liệu vừa nạp ở chế độ Sửa. */
function effectOnChange<V>(read: () => V, action: (value: V) => void): void {
  let initialized = false;
  let prev: V;
  effect(() => {
    const value = read();
    if (!initialized) {
      initialized = true;
      prev = value;
      return;
    }
    if (value === prev) return;
    prev = value;
    action(value);
  });
}

/** 9 control của `buildForm()` bản Reactive Forms cũ. Whitelist BẮT BUỘC: `dialogConfig.data.model` là
 * NGUYÊN bản ghi lịch hẹn (hàng chục key, gồm field quan hệ) — `extractValue()` trần sẽ gửi hết. */
const BASE_PAYLOAD_FIELDS = [
  'checkin_date',
  'checkout_date',
  'noteAppointment',
  'updated_at',
] as const;

interface CheckinFormModel {
  id?: number;
  checkin_date: Date | null;
  checkout_date: Date | null;
  /** bind `<textarea pTextarea>` → seed `null` (gotcha #9 không áp cho TEXTAREA) */
  noteAppointment: string | null;
  updated_at: Date | number | null;
  is_accepted: number | null;
  type_service_accepted_id: number | null;
  appointment_reason_id: number | null;
}

/** Seed đủ mọi field sẽ bind, không field nào `undefined` (gotcha #1). */
function emptyCheckinModel(): CheckinFormModel {
  return {
    checkin_date: null,
    checkout_date: null,
    noteAppointment: null,
    updated_at: null,
    is_accepted: null,
    type_service_accepted_id: null,
    appointment_reason_id: null,
  };
}

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './checkin.html',
  styles: [],
  imports: [...FormShareImports, FormField, AppointmentReasonAutocompleteFormControl],
  standalone: true,
})
export class Checkin extends BaseSignalForm<CheckinFormModel> {
  modelService = inject(AppointmentService) as unknown as BaseApiService<CheckinFormModel> &
    Pick<AppointmentService, 'customerCheckin'>;
  appointment = signal<AppointmentModel>({});
  maxDate: Date = this.timezoneService.todayEnd();
  protected readonly APPOINTMENT_TYPE_SELECT = APPOINTMENT_TYPE_SELECT;

  /** Loại lịch hẹn cố định trong suốt vòng đời dialog → tính 1 lần từ bản ghi truyền vào. */
  protected readonly isTuVan = computed(
    () => this.appointment()?.appointment_type_id === APPOINTMENT_TYPE_SELECT.APPOINTMENT_TYPE_TU_VAN,
  );

  /** 3 trạng thái điều khiển enable/disable + validator động — thay cho `applyAcceptanceRules()` /
   * `applyCheckoutRules()` (chuỗi `enable()/disable()/setValidators()/clearValidators()` của bản cũ). */
  private readonly acceptedValue = computed(() => this.rootField.is_accepted?.().value() ?? null);
  protected readonly typeAcceptedEnabled = computed(
    () => this.isTuVan() && this.acceptedValue() === APPOINTMENT_IS_ACCEPTED_SELECT.APPOINTMENT_IS_ACCEPTED,
  );
  protected readonly reasonEnabled = computed(() => {
    const v = this.acceptedValue();
    return this.isTuVan() && !!v && v !== APPOINTMENT_IS_ACCEPTED_SELECT.APPOINTMENT_IS_ACCEPTED;
  });
  /** Có checkout → `is_accepted` + `checkin_date` bắt buộc (chỉ khi là lịch tư vấn). */
  private readonly hasCheckout = computed(() => !!this.rootField.checkout_date?.().value());

  constructor() {
    super();

    const data = this.dialogConfig?.data?.model;
    this.appointment.set(data);

    // ── Cascade 1: đồng bộ 2 chiều checkin ↔ checkout (bản cũ dùng `valueChanges` +
    // `setValue(..., {emitEvent:false})` để không lặp vô hạn; ở signal thì điều kiện tự hội tụ vì sau khi
    // set xong so sánh không còn đúng nữa). `untracked()` khi GHI để không tự kích hoạt lại effect.
    effectOnChange(
      () => this.rootField.checkin_date?.().value() ?? null,
      checkinDate => {
        untracked(() => {
          const currentCheckout = this.rootField.checkout_date?.().value() ?? null;
          if (checkinDate && currentCheckout && checkinDate > currentCheckout) {
            this.rootField.checkout_date!().value.set(checkinDate);
          }
        });
      },
    );
    effectOnChange(
      () => this.rootField.checkout_date?.().value() ?? null,
      checkoutDate => {
        untracked(() => {
          const currentCheckin = this.rootField.checkin_date?.().value() ?? null;
          if (checkoutDate && currentCheckin && currentCheckin > checkoutDate) {
            this.rootField.checkin_date!().value.set(checkoutDate);
          }
        });
      },
    );

    // ── Cascade 2: đổi `is_accepted` → xoá giá trị của ô vừa bị vô hiệu hoá (bản cũ:
    // `reset(null, {emitEvent:false})` trong `applyAcceptanceRules`). Chỉ chạy khi là lịch tư vấn.
    effectOnChange(
      () => this.acceptedValue(),
      val => {
        if (!this.isTuVan()) return;
        untracked(() => {
          const isAccepted = val === APPOINTMENT_IS_ACCEPTED_SELECT.APPOINTMENT_IS_ACCEPTED;
          if (isAccepted) {
            this.rootField.appointment_reason_id!().value.set(null);
          } else if (val) {
            this.rootField.type_service_accepted_id!().value.set(null);
          } else {
            this.rootField.type_service_accepted_id!().value.set(null);
            this.rootField.appointment_reason_id!().value.set(null);
          }
        });
      },
    );

    // ── Cascade 3: bỏ checkout → xoá `is_accepted` (kéo theo cascade 2 xoá 2 ô phụ).
    // Bản cũ: `applyCheckoutRules(null)` → `isAcceptedCtrl.setValue(null, {emitEvent:true})`.
    effectOnChange(
      () => this.hasCheckout(),
      hasCheckout => {
        if (hasCheckout || !this.isTuVan()) return;
        untracked(() => {
          if (this.rootField.is_accepted!().value() != null) {
            this.rootField.is_accepted!().value.set(null);
          }
        });
      },
    );
  }

  get isAccepted() {
    return this.acceptedValue() === APPOINTMENT_IS_ACCEPTED_SELECT.APPOINTMENT_IS_ACCEPTED;
  }

  get isRejectedOrOther() {
    return this.reasonEnabled();
  }

  get isTuVanGetter() {
    return this.isTuVan();
  }

  /** Override HOÀN TOÀN — bản cũ `initialForm(this.appointment())` rồi `buildForm()` chỉ lấy 9 field,
   * kèm 2 giá trị khởi tạo tính sẵn (`checkin_date` mặc định hôm nay; `checkout_date` không được nhỏ hơn
   * `checkin_date`). Giữ nguyên. */
  override ngOnInit(): void {
    const appt = (this.appointment() ?? {}) as any;

    const initCheckin: Date | null = appt.checkin_date ?? this.today;
    let initCheckout: Date | null = appt.checkout_date ?? null;
    if (initCheckout && initCheckin && initCheckout < initCheckin) {
      initCheckout = new Date(initCheckin.getTime());
    }

    this.model.set({
      ...emptyCheckinModel(),
      checkin_date: initCheckin,
      checkout_date: initCheckout,
      noteAppointment: appt.noteAppointment ?? null,
      updated_at: appt.updated_at ?? null,
      is_accepted: appt.is_accepted ?? null,
      type_service_accepted_id: appt.type_service_accepted_id ?? null,
      appointment_reason_id: appt.appointment_reason_id ?? null,
    });
  }

  /** 2 validator cấp GROUP của bản cũ (`dateRangeValidator`, `inRangeValidator`) chỉ dùng để (a) chặn
   * submit và (b) chọn đúng toast. Signal Forms không có validator cấp group tương đương tiện lợi, nên
   * kiểm tường minh ngay đầu `onSubmit()` — hành vi quan sát được y hệt: chặn submit + đúng toast. */
  private dateRangeInvalid(): boolean {
    const ci = this.rootField.checkin_date!().value();
    const co = this.rootField.checkout_date!().value();
    if (!ci || !co) return false;
    return ci.getTime() > co.getTime();
  }

  private outOfRangeInvalid(): boolean {
    const max = this.maxDate;
    const bad = (d?: Date | null) => !!d && !!max && d.getTime() > max.getTime();
    return bad(this.rootField.checkin_date!().value()) || bad(this.rootField.checkout_date!().value());
  }

  override onSubmit() {
    if (this.isSubmitting()) return;
    this.submitted.set(true);
    this.rootField().markAsTouched();

    if (this.dateRangeInvalid()) {
      this.noti.warnMsg(this.translate.instant('crm.appointment.checkin.error_date_range'));
      return;
    }
    if (this.outOfRangeInvalid()) {
      this.noti.warnMsg(this.translate.instant('crm.appointment.checkin.error_out_of_range'));
      return;
    }

    if (this.rootField().invalid()) {
      // Toast theo field lỗi cụ thể — giữ đúng thứ tự ưu tiên của bản cũ.
      if (this.rootField.is_accepted!().invalid()) {
        this.noti.warnMsg(this.translate.instant('crm.appointment.checkin.error_select_status'));
      } else if (this.rootField.type_service_accepted_id!().invalid()) {
        this.noti.warnMsg(this.translate.instant('crm.appointment.checkin.error_select_service_accepted'));
      } else if (this.rootField.appointment_reason_id!().invalid()) {
        this.noti.warnMsg(this.translate.instant('crm.appointment.checkin.error_select_reason'));
      } else {
        this.noti.errorMsg(this.translate.instant('crm.appointment.checkin.error_check_info'));
      }
      return;
    }

    this.isSubmitting.set(true);
    this.modelService
      .customerCheckin(this.dialogConfig?.data.model.id, this.getPayload())
      .pipe(
        finalize(() => {
          this.isSubmitting.set(false);
        }),
      )
      .subscribe({
        next: (res: any) => {
          const data = res?.data ?? {};
          (data.warnings ?? []).forEach((w: string) => this.noti.warnMsg(w));

          this.noti.successRes(res);
          this.dialogRef.close(data);
        },
      });
  }

  /**
   * Tập key gửi BE **thay đổi theo trạng thái** — bản cũ gửi `rfModel.value`, mà `FormGroup.value`
   * LOẠI key của control đang `disabled`; `extractValue()` của Signal Forms thì VẪN gồm (gotcha R1).
   * Phải tự loại đúng 3 field bị disable động thì tập key mới khớp bản cũ.
   *
   * Ghi chú về mức độ nguy hiểm (đã đọc BE `CustomerCheckin` + `AppointmentService.Checkin`):
   * BE đọc raw body để phân biệt `null` TƯỜNG MINH với key vắng, **nhưng chỉ cho `checkin_date` và
   * `checkout_date`** (null ở 2 field này = rollback cascade, xoá luôn checkout/kết quả tư
   * vấn). Cả 2 field đó bản cũ LUÔN gửi (không bao giờ bị disable) nên hành vi không đổi. Với 3 field
   * disable động thì `null` và "vắng key" cho **cùng kết quả** (`mergeAppointment` bỏ qua con trỏ nil →
   * giữ giá trị DB) — nên việc loại key ở đây là để giữ đúng hợp đồng payload, không phải để vá lỗi.
   */
  protected override getPayload(): any {
    const raw = super.getPayload() as any;
    const payload: any = {};
    for (const key of BASE_PAYLOAD_FIELDS) payload[key] = raw[key];

    // `is_accepted`: bản cũ disable toàn nhóm khi KHÔNG phải lịch tư vấn.
    if (this.isTuVan()) payload.is_accepted = raw.is_accepted;
    // 2 ô phụ: chỉ enable đúng 1 trong 2 tuỳ `is_accepted`.
    if (this.typeAcceptedEnabled()) payload.type_service_accepted_id = raw.type_service_accepted_id;
    if (this.reasonEnabled()) payload.appointment_reason_id = raw.appointment_reason_id;

    return payload;
  }

  protected override buildSchema(p: SchemaPathTree<CheckinFormModel>): void {
    // Disable động — thay chuỗi `enable()/disable({emitEvent:false})` của bản cũ.
    disabled(p.is_accepted!, { when: () => !this.isTuVan() });
    disabled(p.type_service_accepted_id!, { when: () => !this.typeAcceptedEnabled() });
    disabled(p.appointment_reason_id!, { when: () => !this.reasonEnabled() });

    // Validator ĐỘNG — bản cũ dùng `setValidators()/clearValidators()` theo trạng thái. Ở Signal Forms
    // `required()` là tĩnh nên phải viết `validate()` + `requiredError()` có điều kiện.
    // Mỗi điều kiện đã tự gate theo trạng thái enable → field đang disable luôn hợp lệ, đúng như
    // Reactive Forms (control disabled không tham gia validity của group).
    validate(p.is_accepted!, ({ value }) =>
      this.isTuVan() && this.hasCheckout() && value() == null ? requiredError() : null,
    );
    validate(p.checkin_date!, ({ value }) =>
      this.isTuVan() && this.hasCheckout() && value() == null ? requiredError() : null,
    );
    validate(p.type_service_accepted_id!, ({ value }) =>
      this.typeAcceptedEnabled() && value() == null ? requiredError() : null,
    );
    validate(p.appointment_reason_id!, ({ value }) =>
      this.reasonEnabled() && value() == null ? requiredError() : null,
    );
  }
}
