import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { FormField, required, SchemaPathTree } from '@angular/forms/signals';

import { LocationProvinceModel } from '@/core/models/config/location/location-province.model';
import { LocationProvinceService } from '@/core/services/config/location/location-province.service';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import { PROVINCE_TYPE_OPTION } from '@/define';
import { FormShareImports } from '@/share/import/form-share-imports';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-location-province-form',
  templateUrl: './location-province-form.html',
  imports: [...FormShareImports, FormField],
  standalone: true,
})
export class LocationProvinceForm extends BaseSignalForm<LocationProvinceModel> {
  modelService = inject(LocationProvinceService);
  protected readonly PROVINCE_TYPE_OPTION = PROVINCE_TYPE_OPTION;
  // BaseSignalForm không có `blockSpace` (chỉ BaseForm có) — khai riêng tại đây, không sửa base dùng chung.
  readonly blockSpace: RegExp = /[^\s]/;

  protected override buildSchema(p: SchemaPathTree<LocationProvinceModel>): void {
    required(p.name!);
    required(p.Type!);
    required(p.CountryId!);
  }
}
