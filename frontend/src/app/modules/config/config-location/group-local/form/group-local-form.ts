import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { FormField, required, SchemaPathTree } from '@angular/forms/signals';

import { GroupLocalModel } from '@/core/models/config/location/group-local.model';
import { GroupLocalService } from '@/core/services/config/location/group-local.service';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import { FormShareImports } from '@/share/import/form-share-imports';
import { LocationProvinceMultiSelectFormControl } from '@/indexeddb/config/components/location-province';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-group-local-form',
  templateUrl: './group-local-form.html',
  styles: [],
  imports: [...FormShareImports, FormField, LocationProvinceMultiSelectFormControl],
  standalone: true,
})
export class GroupLocalForm extends BaseSignalForm<GroupLocalModel> {
  modelService = inject(GroupLocalService);

  protected override buildSchema(p: SchemaPathTree<GroupLocalModel>): void {
    required(p.name!);
  }
}
