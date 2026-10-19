import { Service} from '@angular/core';
import type { BaseCacheFilterOptions } from '@/indexeddb/config/components/base';
import { BaseCacheService } from '@/indexeddb/config/components/base';
import { AppointmentTypeModel } from '@/core/models/config/appointment/appointment-type.model';

export interface AppointmentTypeFilterOptions extends BaseCacheFilterOptions {
  includeTypeId?: number;
  includeTypeIds?: number[];
}

@Service()
export class AppointmentTypeCacheService extends BaseCacheService<AppointmentTypeModel> {
  protected tableName = 'appointmentTypes';
  protected activeStatus = 1;
  protected allowedStatuses = [1];

  async loadAppointmentTypes() {
    return this.loadItems();
  }

  async ensureAppointmentTypesLoaded(): Promise<void> {
    return this.loadItems();
  }

  getAll(activeOnly = true): AppointmentTypeModel[] {
    const items = this.getAllItems().sort((a, b) => (b.SortOrder ?? 0) - (a.SortOrder ?? 0));
    return activeOnly ? items.filter((item) => item.status === 1) : items;
  }

  protected override generateCacheKey(options: AppointmentTypeFilterOptions): string {
    return `${options.activeOnly}_${options.includeTypeId || ''}_${(options.includeTypeIds || []).join(',')}`;
  }
}
