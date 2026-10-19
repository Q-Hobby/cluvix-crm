import { inject, Service} from '@angular/core';
import { SourceCacheService } from './source-cache.service';
import { SourceModel } from '@/core/models/config/marketing/source.model';

@Service()
export class SourceHelperService {
  private cache = inject(SourceCacheService);

  async ensureSourcesLoaded(): Promise<void> {
    await this.cache.ensureSourcesLoaded();
  }

  mapSourceInfos(ids: number[]): SourceModel[] {
    if (!Array.isArray(ids)) return [];
    return ids
      .filter((id) => id != null)
      .map((id) => this.cache.getById(id))
      .filter((c) => c != null);
  }

  getSourceName(id: number | null | undefined): string {
    if (!id) return '';
    const item = this.cache.getById(id);
    return item?.name || '-';
  }
}
