import { inject, Service } from '@angular/core';
import { LocationProvinceCacheService } from './location-province-cache.service';

@Service()
export class LocationProvinceHelperService {
  private provinceCache = inject(LocationProvinceCacheService);

  async ensureLocationProvincesLoaded() {
    return this.provinceCache.ensureLocationProvincesLoaded();
  }

  getLabel(id: number | null | undefined): string {
    if (!id) return '';
    const province = this.provinceCache.getById(id);
    return province?.name || '';
  }

  getProvinceInfo(id: number | null | undefined) {
    if (!id) return null;
    const province = this.provinceCache.getById(id);
    return {
      code: province!.codeProvince,
      name: province!.name,
    }
  };

  mapProvinceNames(ids: number[]): string {
    if (!ids || ids.length === 0) return '';
    return ids
      .map((id) => this.getLabel(id))
      .filter((name) => name)
      .join(', ');
  }
}
