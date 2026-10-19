import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { FormField, required, SchemaPathTree } from '@angular/forms/signals';

import { CustomerCategoryModel } from '@/core/models/config/marketing/customer-category.model';
import { CustomerCategoryService } from '@/core/services/config/marketing/customer-category.service';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import { FormShareImports } from '@/share/import/form-share-imports';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-customer-category-form',
  templateUrl: './customer-category-form.html',
  styles: [],
  imports: [...FormShareImports, FormField],
  standalone: true,
})
export class CustomerCategoryForm extends BaseSignalForm<CustomerCategoryModel> {
  modelService = inject(CustomerCategoryService);

  changeReminderType(value: number) {
    const typeState = this.rootField.type_nhaclich!();
    if (value == this.modelService.REMINDER_TYPE_GIO) {
      typeState.value.set(this.modelService.REMINDER_TYPE_DAY);
    }
    if (value == this.modelService.REMINDER_TYPE_DAY) {
      typeState.value.set(this.modelService.REMINDER_TYPE_GIO);
    }
  }

  protected override buildSchema(p: SchemaPathTree<CustomerCategoryModel>): void {
    required(p.name!);
  }
}
