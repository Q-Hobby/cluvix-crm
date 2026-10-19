import { Service} from '@angular/core';
import { BaseCacheService } from '@/indexeddb/config/components/base';
import { SettingModel } from '@/core/models/config/system/setting.model';

@Service()
export class SettingCacheService extends BaseCacheService<SettingModel> {
  protected tableName = 'settings';
  protected activeStatus = 1;
  protected allowedStatuses = [1];

  async ensureSettingLoaded(): Promise<void> {
    return this.loadItems();
  }

  getAll(): SettingModel[] {
    return this.getAllItems().filter((item) => item.status === 1);
  }

  protected override generateCacheKey(): string {
    return 'all';
  }
}
