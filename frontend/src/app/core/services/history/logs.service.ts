import { inject, Service} from '@angular/core';
import { Observable } from 'rxjs';
import { BaseApiService } from '@/share/services/base-api.service';
import { UserHelperService } from '@/indexeddb/config/components/user';
import { map } from 'rxjs/operators';
import { ListApiResponse } from '@/share/models/list-api-response';
import { ServiceTypeHelperService } from '@/indexeddb/config/components/service-type/service-type-helper.service';
import { BranchHelperService } from '@/indexeddb/config/components/branch';
import { SourceHelperService } from '@/indexeddb/config/components/source';
import { ServicesHelperService } from '@/indexeddb/config/components/services';
import { CustomerCategoryHelperService } from '@/indexeddb/config/components/customer-category';
import { PaymentTypeHelperService } from '@/indexeddb/config/components/payment-type';
import { PaymentMethodHelperService } from '@/indexeddb/config/components/payment-method';
import { LOG_ACTION_MAP } from '@/define';

@Service()
export class LogsService extends BaseApiService<any> {
  protected override apiModule = 'history';
  protected override endpoint = 'log';
  protected override unixTimesTampFields = ['changed_at'];
  private userHelper: UserHelperService = inject(UserHelperService);
  private serviceTypeHelper = inject(ServiceTypeHelperService);
  private branchHelper = inject(BranchHelperService);
  private sourceHelper = inject(SourceHelperService);
  private servicesHelper = inject(ServicesHelperService);
  private customerCategoryHelper = inject(CustomerCategoryHelperService);
  private paymentTypeHelper = inject(PaymentTypeHelperService);
  private paymentMethodHelper = inject(PaymentMethodHelperService);

  /**
   * @param params
   * @param options
   */
  listLogs(params: any = {}, options: any = {}): Observable<any> {
    const param = params.params.paramUrl;
    params.page = params.page ? (params.page < 1 ? 1 : params.page) : 1;
    return this.apiService
      .get(`${this.path}?_fieldName=${param}`, params)
      .pipe<ListApiResponse<any>>(
        map((listApiResponse: ListApiResponse<any>) => {
          const cloned = JSON.parse(JSON.stringify(listApiResponse));
          cloned.items.map((model: any) => {
            return this.afterLoad(model);
          });
          return cloned;
        }),
      );
  }

  addLabel(item: any) {
    const itemWithDates = this.addFormattedTimestampFields(item, ['changed_at']);
    return {
      ...itemWithDates,
      actionType: LOG_ACTION_MAP[item.action_type],
      changedBy: this.userHelper.getUserInfo(item.changed_by),
      changed_fields: item.changed_fields
        ? Object.entries(item.changed_fields).map(([key, value]) => ({ key, value }))
        : [],
    };
  }

  async addLabels(items: any[]) {
    if (!Array.isArray(items)) return [];
    await this.ensureCacheLoaded();
    return items.map((item) => this.addLabel(item));
  }

  async ensureCacheLoaded() {
    await Promise.all([
      this.userHelper.ensureUsersLoaded(),
      this.branchHelper.ensureBranchesLoaded(),
      this.sourceHelper.ensureSourcesLoaded(),
      this.servicesHelper.ensureServicesLoaded(),
      this.serviceTypeHelper.ensureServiceTypesLoaded(),
      this.customerCategoryHelper.ensureCustomerCategoryLoaded(),
      this.paymentTypeHelper.ensurePaymentTypesLoaded(),
      this.paymentMethodHelper.ensurePaymentMethodsLoaded(),
    ]);
  }
}
