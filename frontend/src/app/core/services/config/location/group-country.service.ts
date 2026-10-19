import { inject, Service} from '@angular/core';

import { GroupCountryModel } from '@/core/models/config/location/group-country.model';
import { BaseApiService } from '@/share/services/base-api.service';
import { LocationCountryHelperService } from '@/indexeddb/config/components/location-country';

@Service()
export class GroupCountryService extends BaseApiService<GroupCountryModel> {
  protected override apiModule = 'config';
  protected override endpoint = 'group-country';
  protected override dateFields = [];
  protected override unixTimesTampFields = ['created_at'];
  private countryHelper = inject(LocationCountryHelperService);

  override getInitialObject() {
    // Signal Forms: field text `pInputText` seed '' (gotcha #9), field mảng seed [] (gotcha #1).
    return {
      name: '',
      desc: null,
      status: 1,
      locationCountryId: [],
    };
  }

  addLabel(item: GroupCountryModel) {
    return {
      ...item,
      countryNames: this.countryHelper.mapLocationCountryNames(item.locationCountryId!),
    };
  }

  async addLabels(items: GroupCountryModel[]) {
    if (!Array.isArray(items)) return [];
    await this.ensureCacheLoaded();
    return items.map((item) => this.addLabel(item));
  }

  async ensureCacheLoaded() {
    await Promise.all([this.countryHelper.ensureLocationCountriesLoaded()]);
  }
}
