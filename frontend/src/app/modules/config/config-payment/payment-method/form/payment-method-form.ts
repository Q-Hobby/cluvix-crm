import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { FormField, required, SchemaPathTree } from '@angular/forms/signals';

import { PaymentMethodModel } from '@/core/models/config/accountant/payment-method.model';
import { PaymentMethodService } from '@/core/services/config/accountant/payment-method.service';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import { FormShareImports } from '@/share/import/form-share-imports';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-payment-method-form',
  templateUrl: './payment-method-form.html',
  styles: [],
  imports: [...FormShareImports, FormField],
  standalone: true,
})
export class PaymentMethodForm extends BaseSignalForm<PaymentMethodModel> {
  modelService = inject(PaymentMethodService);

  protected override buildSchema(p: SchemaPathTree<PaymentMethodModel>): void {
    required(p.name!);
  }
}
