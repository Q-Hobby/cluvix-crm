import { Service} from '@angular/core';

import { AppointmentIsAcceptedModel } from '@/core/models/config/appointment/appointment-is-accepted.model';
import { BaseApiService } from '@/share/services/base-api.service';

@Service()
export class AppointmentIsAcceptedService extends BaseApiService<AppointmentIsAcceptedModel> {
  protected override apiModule = 'config';
  protected override endpoint = 'appointment-is-accepted';
  protected override unixTimesTampFields = ['created_at'];

  // Signal Forms (gotcha #1/#9 `angular22.md`): trả đủ key mọi field `buildSchema()`/template đụng
  // tới, không key nào `undefined`. `name` bind `<input pInputText>` → seed `''`; `severity` (p-select,
  // string) → seed `null`; `desc` bind `<textarea pTextarea>` (không bị gotcha #9) → seed `null`;
  // `isAccepted` giữ default `0` như bản gốc (`model.isAccepted ?? 0`).
  override getInitialObject() {
    return {
      name: '',
      severity: null,
      desc: null,
      isAccepted: 0,
      status: 1,
    };
  }
}
