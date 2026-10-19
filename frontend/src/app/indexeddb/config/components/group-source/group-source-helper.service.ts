import { inject, Service} from '@angular/core';
import { GroupSourceCacheService } from './group-source-cache.service';

@Service()
export class GroupSourceHelperService {
  private cache = inject(GroupSourceCacheService);

  async ensureGroupSourceLoaded(): Promise<void> {
    await this.cache.ensureGroupSourceLoaded();
  }

  getGroupSourceName(id: number | null | undefined): string {
    if (!id) return '';
    const item = this.cache.getById(id);
    return item?.name || '-';
  }
}
