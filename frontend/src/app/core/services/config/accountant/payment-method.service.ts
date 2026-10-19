import { inject, Service} from '@angular/core';

import { PaymentMethodModel } from '@/core/models/config/accountant/payment-method.model';
import { BaseApiService } from '@/share/services/base-api.service';
import { PaymentMethodHelperService } from '@/indexeddb/config/components/payment-method';

@Service()
export class PaymentMethodService extends BaseApiService<PaymentMethodModel> {
  protected override apiModule = 'config';
  protected override endpoint = 'payment-method';
  protected override unixTimesTampFields = ['created_at'];
  private paymentMethodHelper: PaymentMethodHelperService = inject(PaymentMethodHelperService);

  override getInitialObject() {
    return {
      name: '',
      status: 1,
    };
  }

  async ensureCacheLoaded() {
    await Promise.all([this.paymentMethodHelper.ensurePaymentMethodsLoaded()]);
  }
}
