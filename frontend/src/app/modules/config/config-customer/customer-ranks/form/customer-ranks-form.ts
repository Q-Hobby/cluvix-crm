import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { FormField, required, SchemaPathTree } from '@angular/forms/signals';

import { FormShareImports } from '@/share/import/form-share-imports';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import { CustomerRanksModel } from '@/core/models/config/customer/customer-ranks.model';
import { CustomerRanksService } from '@/core/services/config/customer/customer-ranks.service';
import { TAG_SEVERITY_OPTION } from '@/define';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-customer-ranks-form',
  templateUrl: './customer-ranks-form.html',
  styles: [],
  imports: [...FormShareImports, FormField],
  standalone: true,
})
export class CustomerRanksForm extends BaseSignalForm<CustomerRanksModel> {
  modelService = inject(CustomerRanksService);
  protected readonly TAG_SEVERITY_OPTION = TAG_SEVERITY_OPTION;

  protected override buildSchema(p: SchemaPathTree<CustomerRanksModel>): void {
    required(p.name!);
    required(p.min_points!);
  }
}
