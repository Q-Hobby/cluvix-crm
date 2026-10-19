import { inject, Service} from '@angular/core';
import { UserOptionCacheService } from './user-option-cache.service';

@Service()
export class UserOptionHelperService {
  private cache = inject(UserOptionCacheService);

  async ensureSettingLoaded(): Promise<void> {
    await this.cache.ensureUserOptionLoaded();
  }

  getSettingName(id: number | null | undefined): string {
    if (!id) return '';
    const item = this.cache.getById(id);
    return item?.name || '-';
  }

  getUserOptionInfo(id: number) {
    const item = this.cache.getById(id);
    if (!item) return null;
    return {
      id: item.id,
      name: item.name ?? '',
      options: item.options ?? null,
    };
  }
}
