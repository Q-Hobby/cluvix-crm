import { inject, Service } from '@angular/core';
import { LocationCountryCacheService } from './location-country-cache.service';

@Service()
export class LocationCountryHelperService {
  private locationCountryCache = inject(LocationCountryCacheService);

  async ensureLocationCountriesLoaded() {
    return this.locationCountryCache.ensureLocationCountriesLoaded();
  }

  getLabel(id: number): string {
    const locationCountry = this.locationCountryCache.getById(id);
    return locationCountry?.CommonName || '';
  }

  getCountryInfo(id: number) {
    const locationCountry = this.locationCountryCache.getById(id);
    if (!locationCountry) return null;
    return {
      id: locationCountry.id,
      code: locationCountry.CountryCode,
      name: locationCountry.CommonName,
      status: locationCountry.status,
      SortOrder: locationCountry.SortOrder,
    };
  }

  mapLocationCountryNames(ids: number[]): string {
    if (!ids || ids.length === 0) return '';
    return ids
      .map((id) => this.getLabel(id))
      .filter((name) => name)
      .join(', ');
  }
}
