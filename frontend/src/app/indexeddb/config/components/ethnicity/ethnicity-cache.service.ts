import { Service} from '@angular/core';
import { BaseCacheService } from '@/indexeddb/config/components/base';
import {EthnicityModel} from "@/core/models/config/marketing/ethnicity.model";

@Service()
export class EthnicityCacheService extends BaseCacheService<EthnicityModel> {
  protected tableName = 'ethnicity';
  protected activeStatus = 1;
  protected allowedStatuses = [1];

  async ensureEthnicityLoaded(forceReload = false): Promise<void> {
    return this.loadItems(forceReload);
  }

  getAll(): EthnicityModel[] {
    return this.getAllItems()
      .filter((item) => item.status === 1)
      .sort((a, b) => (b.SortOrder ?? 0) - (a.SortOrder ?? 0));
  }

  protected override generateCacheKey(): string {
    return 'all';
  }
}
