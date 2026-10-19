import { Service} from '@angular/core';
import { BaseCacheService } from '@/indexeddb/config/components/base';
import { GroupSourceModel } from '@/core/models/config/marketing/group-source.model';

@Service()
export class GroupSourceCacheService extends BaseCacheService<GroupSourceModel> {
  protected tableName = 'groupSources';
  protected activeStatus = 1;
  protected allowedStatuses = [1];

  async ensureGroupSourceLoaded(forceReload = false): Promise<void> {
    return this.loadItems(forceReload);
  }

  getAll(): GroupSourceModel[] {
    return this.getAllItems()
      .filter((item) => item.status === 1)
      .sort((a, b) => (b.SortOrder ?? 0) - (a.SortOrder ?? 0));
  }

  protected override generateCacheKey(): string {
    return 'all';
  }
}
