import { Service} from '@angular/core';
import { BaseCacheService } from '@/indexeddb/config/components/base';
import { CustomerRankServicesModel } from '@/core/models/config/customer/customer-rank-services.model';

@Service()
export class CustomerRankServicesCacheService extends BaseCacheService<CustomerRankServicesModel> {
  protected tableName = 'customerRankServices';
  protected activeStatus = 1;
  protected allowedStatuses = [1];

  async ensureCustomerRankServicesLoaded(): Promise<void> {
    return this.loadItems();
  }

  getAll(): CustomerRankServicesModel[] {
    return this.getAllItems()
      .filter((item) => item.status === 1)
      .sort((a, b) => (a.id ?? 0) - (b.id ?? 0));
  }

  protected override generateCacheKey(): string {
    return 'all';
  }
}
