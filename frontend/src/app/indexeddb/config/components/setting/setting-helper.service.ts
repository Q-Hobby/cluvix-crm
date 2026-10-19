import { inject, Service} from '@angular/core';
import { SettingCacheService } from './setting-cache.service';

@Service()
export class SettingHelperService {
  private cache = inject(SettingCacheService);

  async ensureSettingLoaded(): Promise<void> {
    await this.cache.ensureSettingLoaded();
  }

  async getSettingByKeyAsync(key: string): Promise<any> {
    try {
      await this.cache.ensureSettingLoaded();
      const settings = this.cache.getAll();
      const found = settings.find((item: any) => item.key === key);
      return found?.value ?? 0;
    } catch (error) {
      console.error('[AuthService] Lỗi lấy setting:', error);
      return 0;
    }
  }
}
