import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { FormField, required, SchemaPathTree } from '@angular/forms/signals';

import { CardTypeModel } from '@/core/models/config/accountant/card-type.model';
import { CardTypeService } from '@/core/services/config/accountant/card-type.service';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import { FormShareImports } from '@/share/import/form-share-imports';
import { PaymentMethodAutocompleteFormControl } from '@/indexeddb/config/components/payment-method';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-card-type-form',
  templateUrl: './card-type-form.html',
  styles: [],
  imports: [...FormShareImports, FormField, PaymentMethodAutocompleteFormControl],
  standalone: true,
})
export class CardTypeForm extends BaseSignalForm<CardTypeModel> {
  modelService = inject(CardTypeService);

  protected override buildSchema(p: SchemaPathTree<CardTypeModel>): void {
    required(p.name!);
    required(p.method!);
  }
}
