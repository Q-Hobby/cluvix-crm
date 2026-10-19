import { inject, Service} from '@angular/core';
import { PaymentMethodCacheService } from './payment-method-cache.service';

@Service()
export class PaymentMethodHelperService {
  private cache = inject(PaymentMethodCacheService);

  async ensurePaymentMethodsLoaded(): Promise<void> {
    await this.cache.ensurePaymentMethodsLoaded();
  }

  getPaymentMethodName(id: number | null | undefined): string {
    if (!id) return '';
    const item = this.cache.getById(id);
    return item?.name || '-';
  }

  mapPaymentMethodNames(ids: number[]): string {
    if (!ids || ids.length === 0) return '';
    return ids
      .map((id) => this.getPaymentMethodName(id))
      .filter((name) => name)
      .join(', ');
  }
}
