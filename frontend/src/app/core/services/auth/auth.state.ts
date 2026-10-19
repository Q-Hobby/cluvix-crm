import { signal } from '@angular/core';

import { BusinessProfileModel } from '@/core/models/config/system/business-profile.model';
import { UserModel } from '@/core/models/hr/user.model';

export const authUser = signal<UserModel | null>(null);
/**
 * Hồ sơ doanh nghiệp (thay `authCompanyActive`/`CompanyModel`, story-43 E6 bỏ company) — 1 dòng DUY
 * NHẤT cho toàn DB (`GET /api/config/business-profile`), KHÔNG theo user/company nào. Populate ở
 * `AuthService.refreshBusinessProfile()`.
 */
export const authBusinessProfile = signal<BusinessProfileModel | null>(null);
export const userPermissions = signal<Set<string>>(new Set());

// Set to true at the end of restoreAuthUser() — guards wait for this before checking permissions.
// Prevents race: withEnabledBlockingInitialNavigation() starts navigation in parallel with
// APP_INITIALIZER, so requirePerm can run before userPermissions is populated.
export const authInitialized = signal(false);
