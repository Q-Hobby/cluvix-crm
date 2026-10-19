import { inject, Service} from '@angular/core';
import { Observable } from 'rxjs';
import { map } from 'rxjs/operators';

import { ApiResponseOneModel } from '@/core/interface/ApiResponseOneModel';
import { ReminderModel } from '@/core/models/crm/reminder.model';
import { BaseApiService } from '@/share/services/base-api.service';
import { ListApiResponse } from '@/share/models/list-api-response';
import { UserHelperService } from '@/indexeddb/config/components/user';
import {
  REMINDER_STATUS_MAP,
  REMINDER_STATUS_SELECT,
  REMINDER_TYPE_MAP,
  REMINDER_TYPE_SELECT,
} from '@/define';

@Service()
export class ReminderService extends BaseApiService<ReminderModel> {
  protected override endpoint = 'reminder';
  protected override unixTimesTampFields = ['start_time', 'created_at', 'updated_at'];
  private userHelper: UserHelperService = inject(UserHelperService);

  /**
   * Khời tạo giá trị ban đầu
   */
  override getInitialObject() {
    return {
      status: REMINDER_STATUS_SELECT.STATUS_PLANED,
      type: REMINDER_TYPE_SELECT.REMINDER_TYPE_CALL,
    };
  }

  /**
   * API lấy danh sách reminder theo customer
   * @param idCustomer
   * @param params
   */
  reminderOfCustomer(idCustomer: number, params: any = {}): Observable<ListApiResponse<ReminderModel>> {
    params.params = {
      ...params.params,
      idCustomer,
    };
    params.page = params.page ? (params.page < 1 ? 1 : params.page) : 1;
    return this.apiService.get<ListApiResponse<ReminderModel>>(`${this.path}/reminder-of-customer`, params).pipe(
      map((res: any) => {
        const cloned = JSON.parse(JSON.stringify(res));
        cloned.items?.forEach((item: any) => this.afterLoad(item));
        return cloned;
      }),
    );
  }

  /**
   * APi tạo mới lời nhắc
   * @param createParams
   * @param options
   */
  createReminder(createParams: ReminderModel, options: any = {}): Observable<ReminderModel> {
    const params = this.beforeSave(createParams);
    return this.apiService.post<any>(`${this.path}`, params, options).pipe(map((res) => res?.data ?? res));
  }

  /**
   * API update lời nhắc
   * @param id
   * @param updateParams
   * @param options
   */
  updateReminder(
    id: number,
    updateParams: ReminderModel,
    options: any = {},
  ): Observable<ApiResponseOneModel<ReminderModel>> {
    const params = this.beforeSave(updateParams);
    return this.apiService
      .put<
        ApiResponseOneModel<ReminderModel>
      >(`${this.path}/${id}`, params, options)
      .pipe(
        map((res) => {
          const cloned = JSON.parse(JSON.stringify(res));
          cloned.data = this.afterLoad(cloned.data);
          return cloned;
        }),
      );
  }

  addLabel(item: ReminderModel) {
    const itemWithDates = this.addFormattedTimestampFields(item, [
      'start_time',
      'created_at',
      'updated_at',
    ]);
    return {
      ...itemWithDates,
      customerDisplayName: `${item?.customer?.fullname || ''}${item?.customer?.nickname ? ' - ' + item?.customer?.nickname : ''}`,
      createdByInfo: this.userHelper.getUserInfo(item?.created_by),
      updatedByInfo: this.userHelper.getUserInfo(item?.updated_by),
      reminderType: REMINDER_TYPE_MAP[item?.type!]!,
      reminderStatus: REMINDER_STATUS_MAP[item?.status!]!,
    };
  }

  async addLabels(items: ReminderModel[]) {
    if (!Array.isArray(items)) return [];
    await this.ensureCacheLoaded();
    return items.map((item) => this.addLabel(item));
  }

  async ensureCacheLoaded() {
    await Promise.all([
      this.userHelper.ensureUsersLoaded(),
    ]);
  }
}
