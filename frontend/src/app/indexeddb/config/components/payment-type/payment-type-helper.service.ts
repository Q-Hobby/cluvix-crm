import { inject, Service} from '@angular/core';
import { PaymentTypeCacheService } from './payment-type-cache.service';

@Service()
export class PaymentTypeHelperService {
  private cache = inject(PaymentTypeCacheService);

  async ensurePaymentTypesLoaded(): Promise<void> {
    await this.cache.ensurePaymentTypesLoaded();
  }

  getPaymentTypeName(id: number | null | undefined): string {
    if (!id) return '';
    const item = this.cache.getById(id);
    return item?.name || '-';
  }

  getPaymentTypesByMethodId(paymentMethodId: number): any[] {
    return this.cache.getByPaymentMethodId(paymentMethodId);
  }
}
