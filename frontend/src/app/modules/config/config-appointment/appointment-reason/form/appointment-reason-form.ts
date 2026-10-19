import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { FormField, required, SchemaPathTree } from '@angular/forms/signals';

import { AppointmentReasonModel } from '@/core/models/config/appointment/appointment-reason.model';
import { AppointmentReasonService } from '@/core/services/config/appointment/appointment-reason.service';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import { FormShareImports } from '@/share/import/form-share-imports';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-appointment-reason-form',
  templateUrl: './appointment-reason-form.html',
  styles: [],
  imports: [...FormShareImports, FormField],
  standalone: true,
})
export class AppointmentReasonForm extends BaseSignalForm<AppointmentReasonModel> {
  modelService = inject(AppointmentReasonService);

  protected override buildSchema(p: SchemaPathTree<AppointmentReasonModel>): void {
    required(p.name!);
  }
}
