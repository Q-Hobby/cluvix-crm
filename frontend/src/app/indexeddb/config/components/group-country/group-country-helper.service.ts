import { inject, Service} from '@angular/core';
import { GroupCountryCacheService } from '@/indexeddb/config/components/group-country/group-country-cache.service';

@Service()
export class GroupCountryHelperService {
  private cache = inject(GroupCountryCacheService);

  async ensureGroupCountryLoaded(): Promise<void> {
    await this.cache.ensureGroupCountryLoaded();
  }

  getGroupCountryName(id: number | null | undefined): string {
    if (!id) return '';
    const item = this.cache.getById(id);
    return item?.name || '-';
  }
}
