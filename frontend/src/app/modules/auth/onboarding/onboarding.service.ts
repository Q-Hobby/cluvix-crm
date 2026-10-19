import { Service, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { map } from 'rxjs/operators';

import { ApiService } from '@/share/services/api.service';

export interface OnboardingStatus {
  can_bootstrap: boolean;
}

export interface OnboardingBootstrapResult {
  branch_id: number | null;
}

/**
 * Client cho `internal/modules/webapp/config/onboarding/` (BE story-01/02) — miễn gate
 * `company_active_required`, KHÔNG extend `BaseApiService` (không phải resource CRUD theo id).
 * Khuôn giống `UserProfileService` (`core/services/config/user-profile.service.ts`): tự ghép
 * `apiUrl` + module + endpoint, gọi `apiService` trực tiếp.
 */
@Service()
export class OnboardingService {
  private readonly apiService = inject(ApiService);
  private readonly path = 'config/onboarding';

  /**
   * `GET /config/onboarding/status`. story-42: caller cũ (màn chọn thương hiệu, gọi khi
   * `!hasCompanies()`) đã bị xoá cùng màn đó — hiện KHÔNG còn caller nào (dead code, giữ lại cho
   * luồng onboarding tương lai dùng `business_profile`, story-63).
   */
  status(): Observable<OnboardingStatus> {
    return this.apiService
      .get<any>(`${this.path}/status`)
      .pipe(map((res) => (res?.data ?? res) as OnboardingStatus));
  }

  /**
   * `POST /config/onboarding/bootstrap` (multipart/form-data). `formData` được dựng sẵn ở nơi gọi
   * (đủ `company`/`branch` JSON string + file tuỳ chọn) — story-06 chịu trách nhiệm build payload
   * đúng contract BE (`dto.CompanyInput`/`dto.BranchInput`).
   */
  bootstrap(formData: FormData): Observable<OnboardingBootstrapResult> {
    return this.apiService
      .post<any>(`${this.path}/bootstrap`, formData)
      .pipe(map((res) => (res?.data ?? res) as OnboardingBootstrapResult));
  }
}
