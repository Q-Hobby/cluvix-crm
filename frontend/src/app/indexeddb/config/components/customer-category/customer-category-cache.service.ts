import { Service} from '@angular/core';
import { BaseCacheService } from '@/indexeddb/config/components/base';
import { CustomerCategoryModel } from '@/core/models/config/marketing/customer-category.model';

@Service()
export class CustomerCategoryCacheService extends BaseCacheService<CustomerCategoryModel> {
  protected tableName = 'customerCategories';
  protected activeStatus = 1;
  protected allowedStatuses = [1];

  async ensureCustomerCategorysLoaded(forceReload = false): Promise<void> {
    return this.loadItems(forceReload);
  }

  getAll(): CustomerCategoryModel[] {
    return this.getAllItems().sort((a, b) => (b.SortOrder ?? 0) - (a.SortOrder ?? 0));
  }

  protected override generateCacheKey(): string {
    return 'all';
  }
}
