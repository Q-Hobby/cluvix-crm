import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { FormField, required, SchemaPathTree } from '@angular/forms/signals';

import { LocationCountryModel } from '@/core/models/config/location/location-country.model';
import { LocationCountryService } from '@/core/services/config/location/location-country.service';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import { FormShareImports } from '@/share/import/form-share-imports';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-config-location-country-form',
  templateUrl: './location-country-form.html',
  imports: [...FormShareImports, FormField],
  standalone: true,
})
export class LocationCountryForm extends BaseSignalForm<LocationCountryModel> {
  modelService = inject(LocationCountryService);
  // BaseSignalForm không có `blockSpace` (chỉ BaseForm có) — khai riêng tại đây, không sửa base dùng chung.
  readonly blockSpace: RegExp = /[^\s]/;

  protected override buildSchema(p: SchemaPathTree<LocationCountryModel>): void {
    required(p.CountryCode!);
    required(p.CommonName!);
  }
}
