import { Service} from '@angular/core';
import { BaseCacheService } from '@/indexeddb/config/components/base';
import { SourceModel } from '@/core/models/config/marketing/source.model';

@Service()
export class SourceCacheService extends BaseCacheService<SourceModel> {
  protected tableName = 'sources';
  protected activeStatus = 1;
  protected allowedStatuses = [1];

  async ensureSourcesLoaded(forceReload = false): Promise<void> {
    return this.loadItems(forceReload);
  }

  getAll(): SourceModel[] {
    return this.getAllItems()
      .filter((item) => item.status === 1)
      .sort((a, b) => (b.SortOrder ?? 0) - (a.SortOrder ?? 0));
  }

  protected override generateCacheKey(): string {
    return 'all';
  }
}
