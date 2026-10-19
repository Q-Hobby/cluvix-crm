import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { finalize } from 'rxjs';
import { FormField, required, SchemaPathTree } from '@angular/forms/signals';

import { AppointmentModel } from '@/core/models/crm/appointment.model';
import { AppointmentService } from '@/core/services/crm/appointment.service';
import { BaseApiService } from '@/share/services/base-api.service';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import { FormShareImports } from '@/share/import/form-share-imports';
import { CONFIRM_APPOINTMENT_OPTION } from '@/define';

/** ĐÚNG 3 control của `buildForm()` bản Reactive Forms cũ. Whitelist BẮT BUỘC: `dialogConfig.data.model`
 * là **nguyên bản ghi lịch hẹn** từ list (rất nhiều key, gồm cả field quan hệ) — nếu để `extractValue()`
 * chạy trần thì payload phình từ 3 key thành hàng chục key. */
const PAYLOAD_FIELDS = ['confirmAppointment', 'noteAppointment', 'updated_at'] as const;

interface ConfirmCheckinFormModel {
  id?: number;
  confirmAppointment: number | null;
  /** bind `<textarea pTextarea>` → seed `null` bình thường (gotcha #9 không áp cho TEXTAREA) */
  noteAppointment: string | null;
  updated_at: Date | number | null;
}

/** Seed đủ mọi field sẽ bind, không field nào `undefined` (gotcha #1). */
function emptyConfirmCheckinModel(): ConfirmCheckinFormModel {
  return {
    confirmAppointment: null,
    noteAppointment: null,
    updated_at: null,
  };
}

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './confirm-checkin.html',
  styles: [],
  imports: [...FormShareImports, FormField],
  standalone: true,
})
export class ConfirmCheckin extends BaseSignalForm<ConfirmCheckinFormModel> {
  modelService = inject(AppointmentService) as unknown as BaseApiService<ConfirmCheckinFormModel> &
    Pick<AppointmentService, 'updateConfirmAppointment'>;
  appointment = signal<AppointmentModel>({});
  protected readonly CONFIRM_APPOINTMENT_OPTION = CONFIRM_APPOINTMENT_OPTION;

  constructor() {
    super();

    const data = this.dialogConfig?.data?.model;
    this.appointment.set(data);
  }

  /** Override HOÀN TOÀN — bản cũ `initialForm(this.appointment())` rồi `buildForm()` chỉ lấy ĐÚNG 3 field
   * từ bản ghi lịch hẹn. Giữ nguyên: model của form CHỈ có 3 field, không phải cả `AppointmentModel`
   * (`BaseSignalForm` constructor đã set `model` = `data.model` — ghi đè lại ở đây cho đúng phạm vi). */
  override ngOnInit(): void {
    const appt = (this.appointment() ?? {}) as any;
    this.model.set({
      ...emptyConfirmCheckinModel(),
      confirmAppointment: appt.confirmAppointment ?? null,
      noteAppointment: appt.noteAppointment ?? null,
      updated_at: appt.updated_at ?? null,
    });
  }

  override onSubmit() {
    if (this.isSubmitting()) return;
    this.submitted.set(true);
    this.rootField().markAsTouched();

    if (this.rootField().invalid()) {
      return;
    }

    this.isSubmitting.set(true);
    this.modelService
      .updateConfirmAppointment(this.dialogConfig?.data.model.id, this.getPayload())
      .pipe(
        finalize(() => {
          this.isSubmitting.set(false);
        }),
      )
      .subscribe({
        next: (res: any) => {
          this.noti.successRes(res);
          this.dialogRef.close(res.data);
        },
        error: (error: any) => {
          this.noti.errorMsg(error.message || this.translate.instant('common.message.error'));
        },
      });
  }

  protected override getPayload(): any {
    const raw = super.getPayload() as any;
    const payload: any = {};
    for (const key of PAYLOAD_FIELDS) payload[key] = raw[key];
    return payload;
  }

  protected override buildSchema(p: SchemaPathTree<ConfirmCheckinFormModel>): void {
    required(p.confirmAppointment!);
  }
}
