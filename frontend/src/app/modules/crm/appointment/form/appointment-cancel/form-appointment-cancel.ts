import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { finalize } from 'rxjs';
import { FormField, required, SchemaPathTree } from '@angular/forms/signals';

import { AppointmentModel } from '@/core/models/crm/appointment.model';
import { AppointmentService } from '@/core/services/crm/appointment.service';
import { BaseApiService } from '@/share/services/base-api.service';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import { FormShareImports } from '@/share/import/form-share-imports';
import { AppointmentReasonAutocompleteFormControl } from '@/indexeddb/config/components/appointment-reason';

/** ĐÚNG 3 control của `buildForm()` bản Reactive Forms cũ. Whitelist tường minh — model của form này
 * KHÔNG chứa gì khác, nhưng giữ để `extractValue()` không kéo theo key lạ nếu model đổi về sau. */
const PAYLOAD_FIELDS = ['appointment_reason_id', 'noteAppointment', 'updated_at'] as const;

interface AppointmentCancelFormModel {
  id?: number;
  appointment_reason_id: number | null;
  /** bind `<textarea pTextarea>` → seed `null` bình thường (gotcha #9 CHỈ áp cho `<input pInputText>`) */
  noteAppointment: string | null;
  updated_at: Date | number | null;
}

/** Seed đủ mọi field sẽ bind, không field nào `undefined` (gotcha #1). */
function emptyAppointmentCancelModel(): AppointmentCancelFormModel {
  return {
    appointment_reason_id: null,
    noteAppointment: null,
    updated_at: null,
  };
}

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './form-appointment-cancel.html',
  styles: [],
  imports: [...FormShareImports, FormField, AppointmentReasonAutocompleteFormControl],
  standalone: true,
})
export class FormAppointmentCancel extends BaseSignalForm<AppointmentCancelFormModel> {
  // `AppointmentService extends BaseApiService<AppointmentModel>` còn form dùng model 3 field riêng.
  modelService = inject(AppointmentService) as unknown as BaseApiService<AppointmentCancelFormModel> &
    Pick<AppointmentService, 'updateAppointmentCancel'>;
  appointment!: AppointmentModel;

  constructor() {
    super();
    this.appointment = this.dialogConfig?.data?.appointment;
  }

  /** Override HOÀN TOÀN — bản cũ `buildForm()` KHÔNG nhận model, luôn dựng form rỗng và chỉ lấy
   * `updated_at` từ `dialogConfig.data.appointment`. Giữ nguyên. */
  override ngOnInit(): void {
    this.model.set({
      ...emptyAppointmentCancelModel(),
      updated_at: this.appointment?.updated_at ?? null,
    });
  }

  override onSubmit() {
    if (this.isSubmitting()) return;
    this.submitted.set(true);
    this.rootField().markAsTouched();

    if (this.rootField().invalid()) return;

    this.isSubmitting.set(true);

    this.modelService
      .updateAppointmentCancel(this.appointment.id, this.getPayload())
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
      });
  }

  protected override getPayload(): any {
    const raw = super.getPayload() as any;
    const payload: any = {};
    for (const key of PAYLOAD_FIELDS) payload[key] = raw[key];
    return payload;
  }

  protected override buildSchema(p: SchemaPathTree<AppointmentCancelFormModel>): void {
    required(p.appointment_reason_id!);
    required(p.noteAppointment!);
  }
}
