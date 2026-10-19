import { Service} from '@angular/core';
import { BaseCacheService } from '@/indexeddb/config/components/base';
import { PaymentMethodModel } from '@/core/models/config/accountant/payment-method.model';

@Service()
export class PaymentMethodCacheService extends BaseCacheService<PaymentMethodModel> {
  protected tableName = 'paymentMethods';
  protected activeStatus = 1;
  protected allowedStatuses = [1];

  async ensurePaymentMethodsLoaded(forceReload = false): Promise<void> {
    return this.loadItems(forceReload);
  }

  getAll(): PaymentMethodModel[] {
    return this.getAllItems()
      .filter((item) => item.status === 1)
      .sort((a, b) => (b.SortOrder ?? 0) - (a.SortOrder ?? 0));
  }

  protected override generateCacheKey(): string {
    return 'all';
  }
}
