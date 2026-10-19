import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { FormField, required, SchemaPathTree } from '@angular/forms/signals';

import { AppointmentNotBookedReasonsModel } from '@/core/models/config/appointment/appointment-not-booked-reasons.model';
import { AppointmentNotBookedReasonsService } from '@/core/services/config/appointment/appointment-not-booked-reasons.service';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import { FormShareImports } from '@/share/import/form-share-imports';
import { AppointmentNotBookedReasonsAutocompleteParentFormControl } from '@/indexeddb/config/components/appointment-not-booked-reasons/appointment-not-booked-reasons-autocomplete-parent-form-control';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-appointment-not-booked-reasons-form',
  templateUrl: './appointment-not-booked-reasons-form.html',
  styles: [],
  imports: [...FormShareImports, FormField, AppointmentNotBookedReasonsAutocompleteParentFormControl],
  standalone: true,
})
export class AppointmentNotBookedReasonsForm extends BaseSignalForm<AppointmentNotBookedReasonsModel> {
  modelService = inject(AppointmentNotBookedReasonsService);

  protected override buildSchema(p: SchemaPathTree<AppointmentNotBookedReasonsModel>): void {
    required(p.name!);
  }
}
