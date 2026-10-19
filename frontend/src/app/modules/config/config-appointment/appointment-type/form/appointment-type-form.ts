import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { FormField, required, SchemaPathTree } from '@angular/forms/signals';

import { AppointmentTypeModel } from '@/core/models/config/appointment/appointment-type.model';
import { AppointmentTypeService } from '@/core/services/config/appointment/appointment-type.service';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import { FormShareImports } from '@/share/import/form-share-imports';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-appointment-type-edit-form',
  templateUrl: './appointment-type-form.html',
  styles: [],
  imports: [...FormShareImports, FormField],
  standalone: true,
})
export class AppointmentTypeForm extends BaseSignalForm<AppointmentTypeModel> {
  modelService = inject(AppointmentTypeService);

  protected override buildSchema(p: SchemaPathTree<AppointmentTypeModel>): void {
    required(p.name!);
  }
}
