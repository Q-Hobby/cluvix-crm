import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { FormField, required, SchemaPathTree } from '@angular/forms/signals';

import { BaseSignalForm } from '@/share/base/base-signal-form';
import { FormShareImports } from '@/share/import/form-share-imports';
import { EthnicityModel } from '@/core/models/config/marketing/ethnicity.model';
import { EthnicityService } from '@/core/services/config/marketing/ethnicity.service';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-ethnicity-form',
  templateUrl: './ethnicity-form.html',
  imports: [...FormShareImports, FormField],
  standalone: true,
})
export class EthnicityForm extends BaseSignalForm<EthnicityModel> {
  modelService = inject(EthnicityService);

  protected override buildSchema(p: SchemaPathTree<EthnicityModel>): void {
    required(p.name!);
  }
}
