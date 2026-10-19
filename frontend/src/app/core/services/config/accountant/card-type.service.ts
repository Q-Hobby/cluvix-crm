import { inject, Service} from '@angular/core';

import { CardTypeModel } from '@/core/models/config/accountant/card-type.model';
import { BaseApiService } from '@/share/services/base-api.service';
import { PaymentMethodHelperService } from '@/indexeddb/config/components/payment-method';

@Service()
export class CardTypeService extends BaseApiService<CardTypeModel> {
  protected override apiModule = 'config';
  protected override endpoint = 'card-type';
  private paymentMethodHelper: PaymentMethodHelperService = inject(PaymentMethodHelperService);

  override getInitialObject() {
    return {
      name: '',
      method: null,
      note: '',
      status: 1,
    };
  }

  addLabel(item: CardTypeModel) {
    return {
      ...item,
      paymentMethodLabel: this.paymentMethodHelper.getPaymentMethodName(item.method),
    };
  }

  async addLabels(items: CardTypeModel[]) {
    if (!Array.isArray(items)) return [];
    await this.ensureCacheLoaded();
    return items.map((item) => this.addLabel(item));
  }

  async ensureCacheLoaded() {
    await Promise.all([this.paymentMethodHelper.ensurePaymentMethodsLoaded()]);
  }
}
