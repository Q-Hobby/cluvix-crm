import { inject, Service} from '@angular/core';

import { AppointmentNotBookedReasonsModel } from '@/core/models/config/appointment/appointment-not-booked-reasons.model';
import { BaseApiService } from '@/share/services/base-api.service';
import { AppointmentNotBookedReasonsHelperService } from '@/indexeddb/config/components/appointment-not-booked-reasons/appointment-not-booked-reasons-helper.service';

@Service()
export class AppointmentNotBookedReasonsService extends BaseApiService<AppointmentNotBookedReasonsModel> {
  protected override apiModule = 'config';
  protected override endpoint = 'appointment-not-booked-reasons';
  protected override unixTimesTampFields = ['created_at', 'updated_at'];
  private appointmentNotBookedReasonsHelper = inject(AppointmentNotBookedReasonsHelperService);

  // Signal Forms (gotcha #1/#9 `angular22.md`): trả đủ key mọi field `buildSchema()`/template đụng
  // tới, không key nào `undefined`. `name` bind `<input pInputText>` → seed `''`; `desc` bind
  // `<textarea pTextarea>` (không bị gotcha #9) → seed `''` cho gọn; `parent_id` (FK, autocomplete
  // CVA control) → seed `null`.
  override getInitialObject() {
    return {
      name: '',
      parent_id: null,
      desc: '',
      status: 1,
    };
  }

  addLabel(item: AppointmentNotBookedReasonsModel) {
    return {
      ...item,
      appointmentNotBookedReasonsLabel: this.appointmentNotBookedReasonsHelper.getReasonName(
        item.parent_id,
      ),
    };
  }

  async addLabels(items: AppointmentNotBookedReasonsModel[]) {
    if (!Array.isArray(items)) return [];
    await this.ensureCacheLoaded();
    return items.map((item) => this.addLabel(item));
  }

  /**
   * Ensure all helper caches are loaded before processing labels
   */
  async ensureCacheLoaded() {
    await Promise.all([this.appointmentNotBookedReasonsHelper.ensureReasonsLoaded()]);
  }
}
