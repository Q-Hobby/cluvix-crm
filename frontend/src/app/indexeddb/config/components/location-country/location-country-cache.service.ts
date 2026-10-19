import { Service} from '@angular/core';
import type { BaseCacheFilterOptions } from '@/indexeddb/config/components/base';
import { BaseCacheService } from '@/indexeddb/config/components/base';
import { LocationCountryModel } from '@/core/models/config/location/location-country.model';

export interface LocationCountryFilterOptions extends BaseCacheFilterOptions {
  includeLocationCountryId?: number;
  includeLocationCountryIds?: number[];
}

@Service()
export class LocationCountryCacheService extends BaseCacheService<LocationCountryModel> {
  protected tableName = 'locationCountries';
  protected activeStatus = 1;
  protected allowedStatuses = [1];

  async loadLocationCountries() {
    return this.loadItems();
  }

  async ensureLocationCountriesLoaded() {
    return this.loadItems();
  }

  getAllLocationCountries(): LocationCountryModel[] {
    return this.getAllItems()
      .filter((item) => item.status === 1)
      .sort((a, b) => (b.SortOrder || 0) - (a.SortOrder || 0));
  }

  filterLocationCountries(options: LocationCountryFilterOptions = {}): LocationCountryModel[] {
    const baseOptions = {
      ...options,
      includeItemId: options.includeLocationCountryId,
      includeItemIds: options.includeLocationCountryIds,
    };

    return this.filterItems(baseOptions);
  }

  protected override generateCacheKey(options: LocationCountryFilterOptions): string {
    return `${options.activeOnly}_${options.includeLocationCountryId || ''}_${(options.includeLocationCountryIds || []).join(',')}`;
  }
}
