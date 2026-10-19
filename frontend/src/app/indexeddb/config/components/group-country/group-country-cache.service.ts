import { Service} from '@angular/core';
import { BaseCacheService } from '@/indexeddb/config/components/base';
import { GroupCountryModel } from '@/core/models/config/location/group-country.model';

@Service()
export class GroupCountryCacheService extends BaseCacheService<GroupCountryModel> {
  protected tableName = 'groupCountries';
  protected activeStatus = 1;
  protected allowedStatuses = [1];

  async ensureGroupCountryLoaded(): Promise<void> {
    return this.loadItems();
  }

  getAll(): GroupCountryModel[] {
    return this.getAllItems()
      .filter((item) => item.status === 1)
      .sort((a, b) => (a.id ?? 0) - (b.id ?? 0));
  }

  protected override generateCacheKey(): string {
    return 'all';
  }
}
