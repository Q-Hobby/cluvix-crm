import { Service} from '@angular/core';

import { CustomerCategoryModel } from '@/core/models/config/marketing/customer-category.model';
import { BaseApiService } from '@/share/services/base-api.service';

@Service()
export class CustomerCategoryService extends BaseApiService<CustomerCategoryModel> {
  protected override apiModule = 'config';
  protected override endpoint = 'customer-category';
  public readonly REMINDER_TYPE_DAY: number = 2; // Ngày
  public readonly REMINDER_TYPE_GIO: number = 1; // Giờ
  protected override unixTimesTampFields = ['created_at'];

  override getInitialObject() {
    return {
      name: '',
      status: 1,
      time_nhaclich: null,
      type_nhaclich: this.REMINDER_TYPE_GIO,
    };
  }

  addLabel(item: CustomerCategoryModel) {
    return { ...item };
  }

  async addLabels(items: CustomerCategoryModel[]) {
    if (!Array.isArray(items)) return [];
    return items.map((item) => this.addLabel(item));
  }
}
