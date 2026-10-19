import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { FormField, required, SchemaPathTree } from '@angular/forms/signals';

import { GroupCountryModel } from '@/core/models/config/location/group-country.model';
import { GroupCountryService } from '@/core/services/config/location/group-country.service';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import { FormShareImports } from '@/share/import/form-share-imports';
import { LocationCountryMultiSelectFormControl } from '@/indexeddb/config/components/location-country/location-country-multiselect-form-control';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-group-country-form',
  templateUrl: './group-country-form.html',
  styles: [],
  imports: [...FormShareImports, FormField, LocationCountryMultiSelectFormControl],
  standalone: true,
})
export class GroupCountryForm extends BaseSignalForm<GroupCountryModel> {
  modelService = inject(GroupCountryService);

  protected override buildSchema(p: SchemaPathTree<GroupCountryModel>): void {
    required(p.name!);
  }
}
