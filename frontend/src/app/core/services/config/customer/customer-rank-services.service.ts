import { inject, Service} from '@angular/core';

import { BaseApiService } from '@/share/services/base-api.service';
import { CustomerRankServicesModel } from '@/core/models/config/customer/customer-rank-services.model';
import { CustomerRanksHelperService } from '@/indexeddb/config/components/customer-ranks';
import { ServicesHelperService } from '@/indexeddb/config/components/services';

@Service()
export class CustomerRankServicesService extends BaseApiService<CustomerRankServicesModel> {
  protected override apiModule = 'config';
  protected override endpoint = 'customer-rank-services';
  protected override unixTimesTampFields = ['created_at'];
  private customerRanksHelper: CustomerRanksHelperService = inject(CustomerRanksHelperService);
  private servicesHelper: ServicesHelperService = inject(ServicesHelperService);

  override getInitialObject() {
    // Signal Forms cần đủ key non-undefined cho mọi field buildSchema()/template đụng tới (gotcha
    // #1/#8 angular22.md) — rank_id/discount_percent số bắt buộc dùng `null` (gotcha #2), service_ids
    // mảng bắt buộc dùng `[]`.
    return {
      rank_id: null,
      discount_percent: null,
      service_ids: [],
      status: 1,
    };
  }

  addLabel(item: CustomerRankServicesModel) {
    return {
      ...item,
      customerRanksInfo: this.customerRanksHelper.getCustomerRankInfo(item.rank_id!),
      servicesInfo: this.servicesHelper.mapServiceInfos(item.service_ids || []),
    };
  }

  async addLabels(items: CustomerRankServicesModel[]) {
    if (!Array.isArray(items)) return [];
    await this.ensureCacheLoaded();
    return items.map((item) => this.addLabel(item));
  }

  async ensureCacheLoaded() {
    await Promise.all([
      this.customerRanksHelper.ensureCustomerRanksLoaded(),
      this.servicesHelper.ensureServicesLoaded(),
    ]);
  }
}
