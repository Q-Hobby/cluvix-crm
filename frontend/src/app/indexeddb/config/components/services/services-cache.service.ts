import { inject, Service } from '@angular/core';
import { BaseCacheService } from '@/indexeddb/config/components/base';
import { ServiceModel } from '@/core/models/config/clinic/service.model';
import { ServiceTypeCacheService } from '@/indexeddb/config/components/service-type/service-type-cache.service';

@Service()
export class ServicesCacheService extends BaseCacheService<ServiceModel> {
  protected tableName = 'services';
  protected activeStatus = 1;
  protected allowedStatuses = [1];

  private serviceTypeSvc = inject(ServiceTypeCacheService);

  async ensureServicesLoaded(forceReload = false): Promise<void> {
    return this.loadItems(forceReload);
  }

  getAll(): ServiceModel[] {
    const activeTypeIds = this.activeServiceTypeIds();
    return this.getAllItems()
      .filter((item) => item.status === 1 && activeTypeIds.has(item.service_type_id!))
      .sort((a, b) => (b.SortOrder || 0) - (a.SortOrder || 0));
  }

  private activeServiceTypeIds(): Set<number> {
    return new Set(this.serviceTypeSvc.getAllServiceTypes().map(st => st.id!));
  }

  /** Trả về tất cả dịch vụ active (status=1) bất kể service type có active không.
   * Dùng cho các màn hình báo cáo/filter lịch sử. */
  getAllActiveIgnoreTypeFilter(): ServiceModel[] {
    return this.getAllItems()
      .filter((item) => item.status === 1)
      .sort((a, b) => (b.SortOrder || 0) - (a.SortOrder || 0));
  }

  getAllServicesUnfiltered(): ServiceModel[] {
    return this.getAllItems();
  }

  protected override generateCacheKey(): string {
    return 'all';
  }
}
