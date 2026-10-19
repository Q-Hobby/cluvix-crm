import { Service} from '@angular/core';

import { AppointmentReasonModel } from '@/core/models/config/appointment/appointment-reason.model';
import { BaseApiService } from '@/share/services/base-api.service';

@Service()
export class AppointmentReasonService extends BaseApiService<AppointmentReasonModel> {
  protected override apiModule = 'config';
  protected override endpoint = 'appointment-reason';
  protected override dateFields = [];
  protected override unixTimesTampFields = ['created_at'];

  // Signal Forms (gotcha #1/#9 `angular22.md`): trả đủ key mọi field `buildSchema()`/template đụng
  // tới, không key nào `undefined`. `name` bind `<input pInputText>` → seed `''`; `time_nhaclich`
  // bind `p-inputnumber` (không phải pInputText) → seed `null` an toàn.
  override getInitialObject() {
    return {
      name: '',
      time_nhaclich: null,
      status: 1,
    };
  }
}
