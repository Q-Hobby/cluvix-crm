import {ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { DynamicDialogConfig, DynamicDialogRef } from 'primeng/dynamicdialog';

import { AppointmentModel } from '@/core/models/crm/appointment.model';
import { FormShareImports } from '@/share/import/form-share-imports';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-appointment-duplicate-dialog',
  templateUrl: './appointment-duplicate.html',
  styles: [],
  standalone: true,
  imports: [...FormShareImports],
})
export class AppointmentDuplicate {
  dialogRef = inject(DynamicDialogRef);
  dialogConfig = inject(DynamicDialogConfig);
  appointment!: AppointmentModel;
  appointmentCreating!: AppointmentModel;
  isDuplicate!: boolean;
  isCreate!: boolean;

  constructor() {
    this.appointment = this.dialogConfig?.data?.appointment;
    this.appointmentCreating = this.dialogConfig?.data?.appointmentCreating;
    this.isDuplicate = this.dialogConfig?.data?.isDuplicate;
    this.isCreate = this.dialogConfig?.data?.isCreate;
  }
}
