import { inject, Service} from '@angular/core';
import { DepartmentCacheService } from '@/indexeddb/config/components/hr-department/department-cache.service';

@Service()
export class DepartmentHelperService {
  private cache = inject(DepartmentCacheService);

  async ensureDepartmentLoaded(): Promise<void> {
    await this.cache.ensureDepartmentLoaded();
  }

  getDepartmentsName(id: number | null | undefined): string {
    if (!id) return '';
    const item = this.cache.getById(id);
    return item?.name || '-';
  }

  getDepartmentsInfo(id: number) {
    const item = this.cache.getById(id);
    if (!item) return null;
    return {
      id: item.id,
      name: item.name,
    };
  }

  mapDepartmentsNames(ids: number | number[]): string {
    const idsArray = Array.isArray(ids) ? ids : [ids];

    if (!idsArray || idsArray.length === 0) return '';
    return idsArray
      .map((id) => this.getDepartmentsName(id))
      .filter((name) => name)
      .join(', ');
  }
}
