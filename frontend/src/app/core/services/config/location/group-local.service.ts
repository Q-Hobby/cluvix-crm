import { inject, Service} from '@angular/core';

import { GroupLocalModel } from '@/core/models/config/location/group-local.model';
import { BaseApiService } from '@/share/services/base-api.service';
import { LocationProvinceHelperService } from '@/indexeddb/config/components/location-province';

@Service()
export class GroupLocalService extends BaseApiService<GroupLocalModel> {
  protected override apiModule = 'config';
  protected override endpoint = 'group-local';
  protected override dateFields = [];
  protected override unixTimesTampFields = ['created_at'];
  private locationProvinceHelper = inject(LocationProvinceHelperService);

  override getInitialObject() {
    // Signal Forms: field text `pInputText` seed '' (gotcha #9), field mảng seed [] (gotcha #1).
    return {
      name: '',
      desc: null,
      status: 1,
      locationProvinceId: [],
    };
  }

  addLabel(item: GroupLocalModel) {
    return {
      ...item,
      provinceNames: this.locationProvinceHelper.mapProvinceNames(item.locationProvinceId!),
    };
  }

  async addLabels(items: GroupLocalModel[]) {
    if (!Array.isArray(items)) return [];
    await this.ensureCacheLoaded();
    return items.map((item) => this.addLabel(item));
  }

  async ensureCacheLoaded() {
    await Promise.all([this.locationProvinceHelper.ensureLocationProvincesLoaded()]);
  }
}
