import { Service} from '@angular/core';

import { BaseApiService } from '@/share/services/base-api.service';
import { CustomerRanksModel } from '@/core/models/config/customer/customer-ranks.model';

@Service()
export class CustomerRanksService extends BaseApiService<CustomerRanksModel> {
  protected override apiModule = 'config';
  protected override endpoint = 'customer-ranks';
  protected override unixTimesTampFields = ['created_at'];

  override getInitialObject() {
    return {
      name: '',
      min_points: null,
      max_points: null,
      discount_percent: null,
      color_code: null,
      description: null,
      status: 1,
    };
  }
}
