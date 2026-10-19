import { Service } from '@angular/core';
import { map, Observable } from 'rxjs';

import { BaseApiService } from '@/share/services/base-api.service';
import { toFormData } from '@/share/services/utils.service';
import { BusinessProfileModel } from '@/core/models/config/system/business-profile.model';

/**
 * `GET/PUT /api/config/business-profile` (BE story-63) — 1 dòng DUY NHẤT, KHÔNG có id trên path,
 * KHÔNG dùng `get(id)`/`update(id, ...)` chung của `BaseApiService` (không phải resource CRUD theo
 * id) — dùng 2 method riêng `getProfile()`/`updateProfile()`, cùng khuôn `createBranch()`/
 * `updateBranch()` (`BranchService`).
 */
@Service()
export class BusinessProfileService extends BaseApiService<BusinessProfileModel> {
  protected override apiModule = 'config';
  protected override endpoint = 'business-profile';

  // BE bọc `{ data: <profile> }` (`response.Data`/`response.Updated`) — PHẢI bóc ở đây: caller
  // (`authBusinessProfile.set`, trang hồ sơ) đọc thẳng field. QA story-43: thiếu bước bóc ⇒ timezone/
  // tiền tệ/logo/tên trên bản in đọc `undefined` và rơi về mặc định toàn app.
  getProfile(): Observable<BusinessProfileModel> {
    return this.apiService.get<{ data: BusinessProfileModel }>(this.path).pipe(map(unwrapProfile));
  }

  /**
   * Partial-update qua allowlist key-presence (BE). `isToFormData=true` khi có `logoFile`
   * (multipart), ngược lại gửi JSON thường. `params.updated_at` PHẢI có mặt — optimistic lock (409
   * `version_conflict` khi lệch bản mới nhất, interceptor chung đã toast).
   */
  updateProfile(params: Record<string, any>, isToFormData = false): Observable<BusinessProfileModel> {
    const body = this.beforeSave(params);
    return this.apiService
      .put<{ data: BusinessProfileModel }>(this.path, isToFormData ? toFormData(body) : body)
      .pipe(map(unwrapProfile));
  }
}

function unwrapProfile(res: { data: BusinessProfileModel } | null | undefined): BusinessProfileModel {
  return (res?.data ?? res) as BusinessProfileModel;
}
