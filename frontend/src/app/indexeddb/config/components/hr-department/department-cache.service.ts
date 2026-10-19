import { Service} from '@angular/core';
import { BaseCacheService } from '@/indexeddb/config/components/base';
import { HrDepartmentModel } from '@/core/models/config/hr/hr-department.model';

@Service()
export class DepartmentCacheService extends BaseCacheService<HrDepartmentModel> {
  protected tableName = 'hrDepartments';
  protected activeStatus = 1;
  protected allowedStatuses = [1];

  async ensureDepartmentLoaded(): Promise<void> {
    return this.loadItems();
  }

  getAll(): HrDepartmentModel[] {
    return this.getAllItems()
      .filter((item) => item.status === 1)
      .sort((a, b) => (a.id ?? 0) - (b.id ?? 0));
  }

  protected override generateCacheKey(): string {
    return 'all';
  }
}
