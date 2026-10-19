import { Service} from '@angular/core';
import type { BaseCacheFilterOptions } from '@/indexeddb/config/components/base';
import { BaseCacheService } from '@/indexeddb/config/components/base';
import { ServiceTypeModel } from '@/core/models/config/clinic/service-type.model';

export interface ServiceTypeFilterOptions extends BaseCacheFilterOptions {
  includeServiceTypeId?: number;
  includeServiceTypeIds?: number[];
}

@Service()
export class ServiceTypeCacheService extends BaseCacheService<ServiceTypeModel> {
  protected tableName = 'serviceTypes';
  protected activeStatus = 1;
  protected allowedStatuses = [1];

  async loadServiceTypes() {
    return this.loadItems();
  }

  async ensureServiceTypesLoaded() {
    return this.loadItems();
  }

  getAllServiceTypes(): ServiceTypeModel[] {
    return this.getAllItems()
      .filter((item) => item.status === 1)
      .sort((a, b) => (b.SortOrder || 0) - (a.SortOrder || 0));
  }

  filterServiceTypes(options: ServiceTypeFilterOptions = {}): ServiceTypeModel[] {
    const baseOptions = {
      ...options,
      includeItemId: options.includeServiceTypeId,
      includeItemIds: options.includeServiceTypeIds,
    };

    return this.filterItems(baseOptions, (serviceTypes, opts) =>
      this.customServiceTypeFilter(serviceTypes, opts as ServiceTypeFilterOptions),
    );
  }

  protected override generateCacheKey(options: ServiceTypeFilterOptions): string {
    const includeIds = Array.isArray(options.includeServiceTypeIds) ? options.includeServiceTypeIds.join(',') : '';
    return `${options.activeOnly}_${options.includeServiceTypeId || ''}_${includeIds}`;
  }

  private customServiceTypeFilter(
    serviceTypes: ServiceTypeModel[],
    options: ServiceTypeFilterOptions,
  ): ServiceTypeModel[] {
    if (options.activeOnly) {
      serviceTypes = serviceTypes.filter((st) => st.status === this.activeStatus);
    } else {
      serviceTypes = serviceTypes.filter((st) => this.allowedStatuses.includes(st.status!));
    }

    return serviceTypes;
  }
}
