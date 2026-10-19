import { inject, Service } from '@angular/core';
import { ServiceTypeCacheService } from './service-type-cache.service';

@Service()
export class ServiceTypeHelperService {
  private serviceTypeCache = inject(ServiceTypeCacheService);

  async ensureServiceTypesLoaded() {
    return this.serviceTypeCache.ensureServiceTypesLoaded();
  }

  getLabel(id: number | null | undefined): string {
    if (!id) return '';
    const serviceType = this.serviceTypeCache.getById(id);
    return serviceType?.name || '';
  }

  mapServiceTypeNames(ids: number[] | string | null | undefined): string {
    // Defensive: BE đôi khi trả JSON string (data legacy double-encoded) thay vì array.
    // Parse string trước khi map để tránh runtime "ids.map is not a function".
    let arr: any = ids;
    if (typeof arr === 'string') {
      try { arr = JSON.parse(arr); } catch { return ''; }
    }
    if (!Array.isArray(arr) || arr.length === 0) return '';
    return arr
      .map((id: any) => this.getLabel(Number(id)))
      .filter((name) => name)
      .join(', ');
  }
}
