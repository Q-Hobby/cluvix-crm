import { Service} from '@angular/core';
import { BaseCacheService } from '@/indexeddb/config/components/base';
import { GroupLocalModel } from '@/core/models/config/location/group-local.model';

@Service()
export class GroupLocalCacheService extends BaseCacheService<GroupLocalModel> {
  protected tableName = 'groupLocals';
  protected activeStatus = 1;
  protected allowedStatuses = [1];

  async ensureGroupLocalLoaded(): Promise<void> {
    return this.loadItems();
  }

  getAll(): GroupLocalModel[] {
    return this.getAllItems()
      .filter((item) => item.status === 1)
      .sort((a, b) => (a.id ?? 0) - (b.id ?? 0));
  }

  protected override generateCacheKey(): string {
    return 'all';
  }
}
