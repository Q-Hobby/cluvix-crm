import { inject, Service} from '@angular/core';
import { GroupLocalCacheService } from '@/indexeddb/config/components/group-local/group-local-cache.service';

@Service()
export class GroupLocalHelperService {
  private cache = inject(GroupLocalCacheService);

  async ensureGroupLocalLoaded(): Promise<void> {
    await this.cache.ensureGroupLocalLoaded();
  }
}
