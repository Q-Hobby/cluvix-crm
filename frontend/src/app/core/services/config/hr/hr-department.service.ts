import { Service} from '@angular/core';

import { HrDepartmentModel } from '@/core/models/config/hr/hr-department.model';
import { BaseApiService } from '@/share/services/base-api.service';

@Service()
export class HrDepartmentService extends BaseApiService<HrDepartmentModel> {
  protected override apiModule = 'config';
  protected override endpoint = 'hr-department';
  protected override dateFields = [];
  protected override unixTimesTampFields = ['created_at'];

  override getInitialObject() {
    // Đủ key cho mọi field `buildSchema()`/template đụng tới (gotcha #8 angular22.md) — `name` seed `''`
    // (bind `pInputText`, gotcha #9); `desc`/`head_user_id` seed `null`.
    return {
      name: '',
      desc: null,
      head_user_id: null,
      status: 1,
    };
  }
}
