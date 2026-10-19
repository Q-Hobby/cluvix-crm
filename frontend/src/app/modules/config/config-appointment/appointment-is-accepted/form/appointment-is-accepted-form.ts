import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { FormField, required, SchemaPathTree } from '@angular/forms/signals';

import { AppointmentIsAcceptedModel } from '@/core/models/config/appointment/appointment-is-accepted.model';
import { AppointmentIsAcceptedService } from '@/core/services/config/appointment/appointment-is-accepted.service';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import { TAG_SEVERITY_OPTION } from '@/define';
import { FormShareImports } from '@/share/import/form-share-imports';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-appointment-is-accepted-form',
  templateUrl: './appointment-is-accepted-form.html',
  styles: [],
  imports: [...FormShareImports, FormField],
  standalone: true,
})
export class AppointmentIsAcceptedForm extends BaseSignalForm<AppointmentIsAcceptedModel> {
  modelService = inject(AppointmentIsAcceptedService);
  protected readonly TAG_SEVERITY_OPTION = TAG_SEVERITY_OPTION;

  protected override buildSchema(p: SchemaPathTree<AppointmentIsAcceptedModel>): void {
    required(p.name!);
    required(p.severity!);
  }
}
