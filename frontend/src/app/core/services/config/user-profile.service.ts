import { Service} from '@angular/core';
import { Observable } from 'rxjs';
import { map } from 'rxjs/operators';

import { UserProfileModel } from '@/core/models/user-profile.model';
import { BaseApiService } from '@/share/services/base-api.service';
import { toFormData } from '@/share/services/utils.service';

@Service()
export class UserProfileService extends BaseApiService<UserProfileModel> {
  protected override apiModule = 'config';
  protected override endpoint = 'user-profile';
  protected override unixTimesTampFields = ['payment_of_date', 'payment_time'];
  protected override dateFields = ['birthday'];

  /**
   * Áp dụng dateFields / unixTimesTampFields lên model đã có sẵn (không qua HTTP).
   * Dùng khi dữ liệu lấy từ signal (authUser) thay vì từ API call.
   */
  processForForm(model: UserProfileModel): UserProfileModel {
    const cloned = JSON.parse(JSON.stringify(model));
    this.afterLoad(cloned);
    return cloned as UserProfileModel;
  }

  /**
   * API update profile user
   * @param paramsValue
   * @param options
   * @param isToFormData
   */
  updateUserProfile(
    paramsValue: any,
    options: any = {},
    isToFormData: any = false,
  ): Observable<UserProfileModel> {
    if (!paramsValue.branchActive_id) {
      paramsValue.branchActive_id = '';
    }
    // ⚠ KHÔNG ép `birthday = ''` khi rỗng (đã gỡ 2026-07-30).
    // `toFormData` append cả key có value rỗng, nên việc ép này nhồi
    // `birthday=''` vào MỌI payload — kể cả màn đổi ngôn ngữ vốn chỉ gửi
    // `locale` (`switch-language.ts` buildForm chỉ có 1 control).
    // Từ 2026-07-30 BE ghi bằng `Updates(map)` — "key có mặt = SET thật" —
    // nên nhồi như cũ sẽ XOÁ ngày sinh của user mỗi lần họ đổi ngôn ngữ.
    // Muốn xoá ngày sinh: form hồ sơ để trống ô đó, key sẽ tự về '' → NULL.
    const params = this.beforeSave(paramsValue);
    return this.apiService.post<any>(
      `${this.path}/update-user-profile`,
      isToFormData ? toFormData(params) : params,
      options,
    ).pipe(map((res) => res?.data ?? res));
  }

  /**
   * API upload avatar
   * @param params
   */
  uploadAvatar(params: any): Observable<UserProfileModel> {
    return this.apiService.post<any>(`${this.path}/create-avatar-user`, params).pipe(map((res) => res?.data ?? res));
  }

  /**
   * API trả về thông tin user profile
   * @param id
   */
  getUserProfile(id: any): Observable<UserProfileModel> {
    return this.apiService
      .get<any>(`${this.path}/get-user-profile?id=${id}`)
      .pipe<UserProfileModel>(
        map((res: any) => {
          const raw = res?.data ?? res;
          const cloned = JSON.parse(JSON.stringify(raw));
          this.afterLoad(cloned);
          return cloned;
        }),
      );
  }

  /**
   * API active thương hiệu user
   * @param param
   */
  switchUserProfile(param: any = {}): Observable<string> {
    return this.apiService.post(`${this.path}/switch-profile`, param);
  }
}
