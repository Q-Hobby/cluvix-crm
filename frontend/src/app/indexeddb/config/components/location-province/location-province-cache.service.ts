import { Service} from '@angular/core';
import type { BaseCacheFilterOptions } from '@/indexeddb/config/components/base';
import { BaseCacheService } from '@/indexeddb/config/components/base';
import { LocationProvinceModel } from '@/core/models/config/location/location-province.model';

export interface LocationProvinceFilterOptions extends BaseCacheFilterOptions {
  countryId?: number;
  includeLocationProvinceId?: number;
  includeLocationProvinceIds?: number[];
}

@Service()
export class LocationProvinceCacheService extends BaseCacheService<LocationProvinceModel> {
  protected tableName = 'locationProvinces';
  protected activeStatus = 1;
  protected allowedStatuses = [1];

  async loadLocationProvinces() {
    return this.loadItems();
  }

  async ensureLocationProvincesLoaded() {
    return this.loadItems();
  }

  getAllLocationProvinces(): LocationProvinceModel[] {
    return this.getAllItems()
      .filter((item) => item.status === 1)
      .sort((a, b) => (b.SortOrder || 0) - (a.SortOrder || 0));
  }

  filterLocationProvinces(options: LocationProvinceFilterOptions = {}): LocationProvinceModel[] {
    const baseOptions = {
      ...options,
      includeItemId: options.includeLocationProvinceId,
      includeItemIds: options.includeLocationProvinceIds,
    };

    return this.filterItems(baseOptions, (provinces, opts) =>
      this.customProvinceFilter(provinces, opts as LocationProvinceFilterOptions),
    );
  }

  protected override generateCacheKey(options: LocationProvinceFilterOptions): string {
    return `${options.activeOnly}_${options.countryId || ''}_${options.includeLocationProvinceId || ''}_${(options.includeLocationProvinceIds || []).join(',')}`;
  }

  private customProvinceFilter(
    provinces: LocationProvinceModel[],
    options: LocationProvinceFilterOptions,
  ): LocationProvinceModel[] {
    if (options.countryId) {
      provinces = provinces.filter((p) => p.CountryId === options.countryId);
    }

    return provinces;
  }
}
