import { Service} from '@angular/core';
import { BaseCacheService } from '@/indexeddb/config/components/base';
import { UserOptionModel } from '@/core/models/config/system/user-option.model';

@Service()
export class UserOptionCacheService extends BaseCacheService<UserOptionModel> {
  protected tableName = 'userOptions';
  protected activeStatus = 1;
  protected allowedStatuses = [1];

  async ensureUserOptionLoaded(): Promise<void> {
    return this.loadItems();
  }

  getAll(): UserOptionModel[] {
    return this.getAllItems().filter((item) => item.status === 1);
  }

  protected override generateCacheKey(): string {
    return 'all';
  }
}
