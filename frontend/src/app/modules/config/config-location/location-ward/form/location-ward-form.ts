import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { FormField, required, SchemaPathTree } from '@angular/forms/signals';

import { LocationWardModel } from '@/core/models/config/location/location-ward.model';
import { LocationWardService } from '@/core/services/config/location/location-ward.service';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import { WARD_TYPE_OPTION } from '@/define';
import { FormShareImports } from '@/share/import/form-share-imports';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-location-ward-form',
  templateUrl: './location-ward-form.html',
  imports: [...FormShareImports, FormField],
  standalone: true,
})
export class LocationWardForm extends BaseSignalForm<LocationWardModel> {
  modelService = inject(LocationWardService);
  protected readonly WARD_TYPE_OPTION = WARD_TYPE_OPTION;

  protected override buildSchema(p: SchemaPathTree<LocationWardModel>): void {
    required(p.name!);
    required(p.Type!);
    required(p.province_id!);
  }
}
