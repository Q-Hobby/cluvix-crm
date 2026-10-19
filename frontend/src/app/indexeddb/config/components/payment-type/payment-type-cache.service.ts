import { Service} from '@angular/core';
import { BaseCacheService } from '@/indexeddb/config/components/base';
import { CardTypeModel } from '@/core/models/config/accountant/card-type.model';

@Service()
export class PaymentTypeCacheService extends BaseCacheService<CardTypeModel> {
  protected tableName = 'paymentTypes';
  protected activeStatus = 1;
  protected allowedStatuses = [1];

  async ensurePaymentTypesLoaded(forceReload = false): Promise<void> {
    return this.loadItems(forceReload);
  }

  getAll(): CardTypeModel[] {
    return this.getAllItems()
      .filter((item) => item.status === 1)
      .sort((a, b) => (b.SortOrder ?? 0) - (a.SortOrder ?? 0));
  }

  getByPaymentMethodId(paymentMethodId: number): CardTypeModel[] {
    return this.getAllItems()
      .filter((item) => item.method === paymentMethodId && item.status === 1)
      .sort((a, b) => (b.SortOrder ?? 0) - (a.SortOrder ?? 0));
  }

  protected override generateCacheKey(): string {
    return 'all';
  }
}
