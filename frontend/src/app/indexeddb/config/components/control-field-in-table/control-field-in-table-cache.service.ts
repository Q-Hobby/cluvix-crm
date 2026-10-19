import { Service} from '@angular/core';
import { BaseCacheService } from '@/indexeddb/config/components/base';
import { CustomizeFormModel } from '@/core/models/config/customize-form/customize-form.model';

@Service()
export class ControlFieldInTableCacheService extends BaseCacheService<CustomizeFormModel> {
  protected tableName = 'controlFieldInTables';
  protected activeStatus = 1;
  protected allowedStatuses = [1];

  async ensureControlFieldInTableLoaded(forceReload = false): Promise<void> {
    return this.loadItems(forceReload);
  }

  getAll(): CustomizeFormModel[] {
    return this.getAllItems();
  }

  protected override generateCacheKey(): string {
    return 'all';
  }
}
