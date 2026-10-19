import { Service} from '@angular/core';
import { BaseCacheService } from '@/indexeddb/config/components/base';
import {JobModel} from "@/core/models/config/marketing/job.model";

@Service()
export class JobCacheService extends BaseCacheService<JobModel> {
  protected tableName = 'job';
  protected activeStatus = 1;
  protected allowedStatuses = [1];

  async ensureJobLoaded(forceReload = false): Promise<void> {
    return this.loadItems(forceReload);
  }

  getAll(): JobModel[] {
    return this.getAllItems()
      .filter((item) => item.status === 1)
      .sort((a, b) => (b.SortOrder ?? 0) - (a.SortOrder ?? 0));
  }

  protected override generateCacheKey(): string {
    return 'all';
  }
}
