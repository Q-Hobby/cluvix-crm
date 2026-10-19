import { Service} from '@angular/core';
import { BaseCacheService } from '@/indexeddb/config/components/base';
import { CustomerRanksModel } from '@/core/models/config/customer/customer-ranks.model';

@Service()
export class CustomerRanksCacheService extends BaseCacheService<CustomerRanksModel> {
  protected tableName = 'customerRanks';
  protected activeStatus = 1;
  protected allowedStatuses = [1];

  async ensureCustomerRanksLoaded(): Promise<void> {
    return this.loadItems();
  }

  getAll(): CustomerRanksModel[] {
    return this.getAllItems()
      .filter((item) => item.status === 1)
      .sort((a, b) => (a.id ?? 0) - (b.id ?? 0));
  }

  protected override generateCacheKey(): string {
    return 'all';
  }
}
