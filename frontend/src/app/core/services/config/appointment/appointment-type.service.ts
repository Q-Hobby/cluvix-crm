import { Service} from '@angular/core';

import { AppointmentTypeModel } from '@/core/models/config/appointment/appointment-type.model';
import { BaseApiService } from '@/share/services/base-api.service';
import { Observable } from 'rxjs';

@Service()
export class AppointmentTypeService extends BaseApiService<AppointmentTypeModel> {
  protected override apiModule = 'config';
  protected override endpoint = 'appointment-type';
  protected override dateFields = [];
  protected override unixTimesTampFields = ['created_at'];

  // Signal Forms (gotcha #1/#9 `angular22.md`): trả đủ key mọi field `buildSchema()`/template đụng
  // tới, không key nào `undefined`. `name` bind `<input pInputText>` → seed `''`.
  override getInitialObject() {
    return {
      name: '',
      status: 1,
    };
  }

  handleTypeRevenueChange(params: any): Observable<string> {
    return this.apiService.post(`${this.path}/type-revenue-change`, params, {});
  }
}
