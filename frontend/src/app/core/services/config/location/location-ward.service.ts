import { inject, Service} from '@angular/core';
import { SelectItem } from 'primeng/api';
import { Observable } from 'rxjs';

import { LocationWardModel } from '@/core/models/config/location/location-ward.model';
import { BaseApiService } from '@/share/services/base-api.service';
import { LocationProvinceHelperService } from '@/indexeddb/config/components/location-province/location-province-helper.service';

@Service()
export class LocationWardService extends BaseApiService<LocationWardModel> {
  protected override apiModule = 'config';
  protected override endpoint = 'location-ward';
  protected override unixTimesTampFields = ['created_at'];
  private locationProvinceHelper: LocationProvinceHelperService = inject(
    LocationProvinceHelperService,
  );

  override getInitialObject() {
    // Signal Forms: field text `pInputText` seed '' (gotcha #9); field số/reference seed null (gotcha #1).
    return {
      codeWard: '',
      name: '',
      Type: null,
      province_id: null,
      status: 1 as any,
    };
  }

  getProvince(provinceId: any): Observable<SelectItem[]> {
    return this.apiService.get(this.path + `/get-by-province?province_id=${provinceId}`);
  }

  addLabel(item: LocationWardModel) {
    return {
      ...item,
      provinceLabel: this.locationProvinceHelper.getLabel(item.province_id),
    };
  }

  async addLabels(items: LocationWardModel[]) {
    if (!Array.isArray(items)) return [];
    await this.ensureCacheLoaded();
    return items.map((item) => this.addLabel(item));
  }

  /**
   * Ensure all helper caches are loaded before processing labels
   */
  async ensureCacheLoaded() {
    await Promise.all([this.locationProvinceHelper.ensureLocationProvincesLoaded()]);
  }
}
