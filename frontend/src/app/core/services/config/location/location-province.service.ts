import { inject, Service} from '@angular/core';

import { LocationProvinceModel } from '@/core/models/config/location/location-province.model';
import { BaseApiService } from '@/share/services/base-api.service';
import { LocationCountryHelperService } from '@/indexeddb/config/components/location-country/location-country-helper.service';

@Service()
export class LocationProvinceService extends BaseApiService<LocationProvinceModel> {
  protected override apiModule = 'config';
  protected override endpoint = 'location-province';
  protected override unixTimesTampFields = ['created_at'];
  private locationCountryHelper: LocationCountryHelperService = inject(
    LocationCountryHelperService,
  );

  override getInitialObject() {
    // Signal Forms: field text `pInputText` seed '' (gotcha #9); field số/reference seed null (gotcha #1).
    return {
      codeProvince: '',
      name: '',
      Type: null,
      status: 1,
      TelephoneCode: null,
      CountryId: null,
      CountryCode: null,
    };
  }

  addLabel(item: LocationProvinceModel) {
    return {
      ...item,
      countryInfo: this.locationCountryHelper.getCountryInfo(item?.CountryId!),
    };
  }

  async addLabels(items: LocationProvinceModel[]) {
    if (!Array.isArray(items)) return [];
    await this.ensureCacheLoaded();
    return items.map((item) => this.addLabel(item));
  }

  /**
   * Ensure all helper caches are loaded before processing labels
   */
  async ensureCacheLoaded() {
    await Promise.all([this.locationCountryHelper.ensureLocationCountriesLoaded()]);
  }
}
