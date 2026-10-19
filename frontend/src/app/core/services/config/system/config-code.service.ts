import { inject, Service} from '@angular/core';

import { map, Observable } from 'rxjs';

import { ConfigCodeModel, CustomerCodeOptionInfo } from '@/core/models/config/system/config-code.model';
import { BaseApiService } from '@/share/services/base-api.service';
import { BranchHelperService } from '@/indexeddb/config/components/branch';

@Service()
export class ConfigCodeService extends BaseApiService<ConfigCodeModel> {
  protected override apiModule = 'config';
  protected override endpoint = 'config-code';
  protected override unixTimesTampFields = ['created_at', 'updated_at'];
  private branchHelper: BranchHelperService = inject(BranchHelperService);

  override getInitialObject() {
    return {
      name: '',
      parent_table: '',
      infix_code: '',
      include_branch_code: true,
      monthly_reset: true,
      serial_length: 2,
    };
  }

  getCustomerCodeOption(): Observable<CustomerCodeOptionInfo> {
    return this.apiService
      .get<{ data: CustomerCodeOptionInfo }>(`${this.path}/customer-code-option`)
      .pipe(map((res) => res.data));
  }

  addLabel(item: ConfigCodeModel) {
    return {
      ...item,
      branchInfo: this.branchHelper.getBranchInfo(item.branch_id),
    };
  }

  async addLabels(items: ConfigCodeModel[]) {
    if (!Array.isArray(items)) return [];
    await this.ensureCacheLoaded();
    return items.map((item) => this.addLabel(item));
  }

  async ensureCacheLoaded() {
    await Promise.all([
      this.branchHelper.ensureBranchesLoaded(),
    ]);
  }
}
