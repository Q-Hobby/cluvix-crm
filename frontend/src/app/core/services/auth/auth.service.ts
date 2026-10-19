import {effect, inject, Service, OnDestroy} from '@angular/core';
import {HttpContext} from '@angular/common/http';
import {Router} from '@angular/router';
import {firstValueFrom, Observable, of} from 'rxjs';
import {catchError, finalize, switchMap} from 'rxjs/operators';
import {authBusinessProfile, authInitialized, authUser, userPermissions, RbacService} from '@/core/services/auth';
import {TokenService} from './token.service';
import {UserAuthSessionService} from './user-auth-session.service';
import {STORAGE_KEY_SELECT} from '@/define';
import {BaseApiService} from '@/share/services/base-api.service';
import {StorageService} from '@/share/services/storage.service';
import {NotificationService} from '@/share/services/notification.service';
import {BusinessProfileService} from '@/core/services/config/system/business-profile.service';
import {GlobalLoadingService} from '@/share/services/global-loading.service';
import {CacheManagerService} from '@/core/services/cache/cache-manager.service';
import {CrmDashboardDataCache} from '@/core/services/crm/crm-dashboard-data-cache.service';
import {SKIP_ERROR_NOTIFICATION} from '@/core/interceptors/response-handler.interceptor';

@Service()
export class AuthService extends BaseApiService<any> implements OnDestroy {
  protected route: Router = inject(Router);
  protected override dateFields: any = ['birthday'];

  protected override get path(): string {
    return 'auth';
  }

  private businessProfileService = inject(BusinessProfileService);
  private cacheManager = inject(CacheManagerService);
  private globalLoadingService = inject(GlobalLoadingService);
  private noti = inject(NotificationService);
  private tokenService = inject(TokenService);
  private rbacService = inject(RbacService);
  private storageService = inject(StorageService);
  private userAuthSession = inject(UserAuthSessionService);
  // Dashboard CRM tuỳ biến: số liệu khối cache trong bộ nhớ phiên — xoá cùng chỗ xoá token (máy quầy dùng chung).
  private crmDashboardDataCache = inject(CrmDashboardDataCache);
  private isLoggingOut = false;
  // Cache authVersion in-memory để tránh đọc IDB async trong effect mỗi lần authUser update.
  // Giá trị luôn phản ánh version đã save vào IDB gần nhất.
  _cachedAuthVersion = 0;
  private _fetchMeInFlight = false;

  constructor() {
    super();
    this.setupStorageListener();
    this.setupRefreshTokenListener();
    this.setupAuthSync();
    // restoreAuthUser() được gọi từ APP_INITIALIZER SAU KHI tokenService.initialize()
    // hoàn tất — tránh race condition đọc signals chưa populate.
  }

  ngOnDestroy(): void {
    try {
      window.removeEventListener('refreshTokenExpired', this.refreshTokenHandler);
      window.removeEventListener('storage', this.handleStorageChange);
    } catch (err) {
      console.error('[AuthService] Cleanup error:', err);
    }
  }


  // AUTH FLOW
  login(params: any): Observable<any> {
    // ApiService mặc định withCredentials=true → BE Set-Cookie refresh_token (HttpOnly) sẽ được browser store
    // SKIP_ERROR_NOTIFICATION: component tự hiển thị lỗi (sai mật khẩu / rate-limit
    // đếm ngược) — tránh interceptor toast trùng (interceptor cũng tự toast 401 khi đang ở /login).
    return this.apiService
      .post(`${this.path}/login`, params, { context: new HttpContext().set(SKIP_ERROR_NOTIFICATION, true) })
      .pipe(switchMap((body: any) => this.handleLoginBody(body)));
  }

  /** Bước 2 của 2FA — verify mã rồi cấp token (cùng xử lý body như login). */
  login2fa(params: { challenge_token: string; code: string; remember_device: boolean }): Observable<any> {
    return this.apiService
      .post(`${this.path}/login/2fa`, params, { context: new HttpContext().set(SKIP_ERROR_NOTIFICATION, true) })
      .pipe(switchMap((body: any) => this.handleLoginBody(body)));
  }

  /** Xử lý body login/login2fa: set token + user + permission + session. */
  private async handleLoginBody(body: any): Promise<any> {
    // requires_2fa: chưa có token — component sẽ chuyển sang màn nhập mã.
    if (body?.requires_2fa) return body;

    // Await setTokens trước — đảm bảo encrypted access_token đã ghi vào localStorage
    // trước khi authUser.set() trigger storage event cho các tab khác.
    if (body?.accessToken && typeof body.accessToken === 'string') {
      await this.tokenService.setTokens(body.accessToken, body.expiresAt, body.refreshExpiresAt);
    }
    if (body?.user && typeof body.user === 'object') {
      authUser.set(body.user);
    }
    if (body?.permission && Array.isArray(body.permission)) {
      this.rbacService.setPermissions(body.permission);
    }
    const userId = body?.user?.id;
    if (userId) {
      const permStrings = this.rbacService.getPermissionStrings();
      await this.userAuthSession.save({ userId, user: body.user, permissions: permStrings, authVersion: 0 });
      this.storageService.saveStorage('auth_user_id', userId);
    }
    return body;
  }

  // 2FA (TOTP) management
  get2FAStatus(): Observable<{ enabled: boolean }> {
    return this.apiService.get(`${this.path}/2fa/status`);
  }
  setup2FA(): Observable<{ uri: string; secret: string }> {
    return this.apiService.post(`${this.path}/2fa/setup`, {});
  }
  verifySetup2FA(code: string): Observable<{ backup_codes: string[] }> {
    return this.apiService.post(`${this.path}/2fa/verify-setup`, { code });
  }
  disable2FA(code: string): Observable<any> {
    return this.apiService.post(`${this.path}/2fa/disable`, { code });
  }
  reset2FA(userId: number): Observable<any> {
    return this.apiService.post(`${this.path}/2fa/reset/${userId}`, {});
  }

  logout(): Observable<boolean> {
    if (this.isLoggingOut) {
      return of(true);
    }

    this.isLoggingOut = true;
    this.globalLoadingService.showLoading('logout');

    // ApiService mặc định withCredentials=true → cookie tự gửi để BE consume + Set-Cookie clear.
    // Cleanup luôn chạy dù API thành công hay fail (BE 401/network down) — FE không thể stuck.
    return this.apiService.post(`${this.path}/logout`, {}).pipe(
      catchError((err) => {
        console.error('[AuthService] Logout API failed:', err);
        return of(null);
      }),
      switchMap(async () => {
        await this.clearAllAuthData();
        this.tokenService.clearToken();
        // Không set authUser = null để tránh UI flicker — location.href = '/login' reload toàn bộ.
        window.location.href = '/login';
        return true;
      }),
      finalize(() => {
        this.isLoggingOut = false;
        // hideLoading không cần thiết vì page reload ngay sau đó
      }),
    );
  }

  public async clearAllAuthData(): Promise<void> {
    // Clear storage nhưng KHÔNG clear authUser signal để tránh UI flicker
    this.rbacService.clearPermissions();
    this.storageService.clearOnLogout();
    this.crmDashboardDataCache.clear();
    try {
      await this.cacheManager.clearIndexedDB();
    } catch (error) {
      console.error('[AuthService] Error clearing IndexedDB:', error);
      this.noti?.warn?.('Không thể xóa hoàn toàn dữ liệu local');
    }
  }

  public handleRefreshTokenExpired(): void {
    if (this.isLoggingOut) return;

    this.isLoggingOut = true;

    this.clearAllAuthData()
      .then(() => {
        this.tokenService.clearToken();
        authUser.set(null);
      })
      .catch((err) => {
        console.error('[AuthService] Session expired cleanup failed:', err);
        this.tokenService.clearToken();
        authUser.set(null);
      })
      .finally(() => {
        this.isLoggingOut = false;
        // Chỉ redirect nếu chưa ở trang login
        const currentUrl = window.location.pathname;
        if (!currentUrl.startsWith('/login')) {
          window.location.href = '/login?reason=session-expired';
        }
      });
  }

  deleteCache(): Observable<string> {
    return this.apiService.post(`${this.path}/clear-cache`);
  }

  /** Danh sách thiết bị đã đăng nhập của user hiện tại (device binding). */
  listDevices(): Observable<any[]> {
    return this.apiService.get(`${this.path}/devices`);
  }

  /** Đăng xuất 1 thiết bị (xoá refresh token + đánh dấu revoked). */
  revokeDevice(id: number): Observable<any> {
    return this.apiService.post(`${this.path}/devices/${id}/revoke`, {});
  }

  /** Nhật ký bảo mật (super-user, scope theo company active). Trả {items, _meta}. */
  getAuditLog(params: Record<string, any>): Observable<any> {
    const qs = new URLSearchParams();
    Object.entries(params).forEach(([k, v]) => {
      if (v !== null && v !== undefined && v !== '') qs.set(k, String(v));
    });
    return this.apiService.get(`${this.path}/audit-log?${qs.toString()}`);
  }

  private refreshTokenHandler = () => {
    if (!this.isLoggingOut) {
      this.handleRefreshTokenExpired();
    }
  };

  private setupAuthSync(): void {
    // Skip lần đầu: effect chạy với authUser() = null trong APP_INITIALIZER (trước khi
    // restoreAuthUser kịp set), sẽ xóa nhầm `auth_user` localStorage. restoreAuthUser
    // tự đọc/ghi localStorage, ta chỉ cần phản ứng các thay đổi tiếp theo.
    let isFirstRun = true;

    effect(() => {
      const user = authUser();

      if (isFirstRun) {
        isFirstRun = false;
        return;
      }

      if (this.isLoggingOut) return;

      if (user) {
        // Cập nhật IndexedDB — giữ nguyên authVersion hiện tại (không reset khi profile thay đổi).
        // Đọc permissions từ signal (in-memory) — không đọc localStorage để tránh stale data
        // sau restore từ IndexedDB (setPermissionsFromStrings không ghi localStorage).
        const userId = user.id;
        if (userId) {
          const permStrings = Array.from(userPermissions());
          const currentAuthVersion = this._cachedAuthVersion;
          void this.userAuthSession.save({
            userId,
            user,
            permissions: permStrings,
            authVersion: currentAuthVersion,
          });
          this.storageService.saveStorage('auth_user_id', userId);
        }
      } else {
        this.storageService.removeStorage('auth_user_id');
      }
    });
  }

  /**
   * (Re)load `authBusinessProfile` — hồ sơ doanh nghiệp 1 dòng DUY NHẤT cho toàn DB (story-43, thay
   * `authCompanyActive`/`refreshCompanyActive`). KHÔNG theo user/company nào nên gọi vô điều kiện,
   * không cần track "id đổi" như company cũ. Gọi ở APP_INITIALIZER (`app.config.ts`) và sau login
   * (`syncing.ts`) — lỗi mạng thoáng qua thì giữ nguyên signal cũ, không set null (tránh flicker).
   */
  public async refreshBusinessProfile(): Promise<void> {
    try {
      const profile = await firstValueFrom(this.businessProfileService.getProfile());
      authBusinessProfile.set(profile || null);
    } catch (err) {
      console.error('[AuthService] Load business profile failed:', err);
    }
  }

  /** Public async — gọi từ APP_INITIALIZER, await để đảm bảo authUser set trước preloadIndexedDb. */
  public async restoreAuthUser(): Promise<void> {
    try {
      const hasToken = this.tokenService.hasValidToken;
      const hasRefreshToken = this.tokenService.hasValidRefreshSession();

      // Đọc userId từ localStorage (lightweight marker) để load đúng IndexedDB entry
      const userId = this.storageService.loadStorage('auth_user_id') as number | null;

      // Thử load từ IndexedDB trước
      let session = userId ? await this.userAuthSession.load(userId) : null;

      // Fallback: session cũ từ localStorage (migration cho user chưa login lại)
      const lsCached = !session ? this.storageService.loadStorage(STORAGE_KEY_SELECT.AuthUser) : null;
      const hasCached = !!(session || lsCached);

      // Case 1a: Refresh session còn + thiếu user data (IDB bị xóa do switch company / clear cache).
      // Cover CẢ khi access token đã hết hạn (silent refresh trước) — nếu chỉ check hasToken,
      // F5 sau >15 phút với IDB rỗng sẽ lọt xuống không case nào: authGuard tự refresh OK nhưng
      // authUser/userPermissions rỗng → accessdenied giả trên mọi deep route.
      // → Populate authUser + userPermissions từ BE trước khi guards chạy.
      // Không redirect vì session vẫn valid — user ở lại trang hiện tại.
      if (!hasCached && hasRefreshToken) {
        console.warn('[AuthService] No cached user (IDB cleared) — refreshing session + fetching /me');
        const tokenOk = hasToken || (await this.tokenService.trySilentRefresh());
        if (tokenOk) {
          await this.fetchAndSyncMe();
        }
        // tokenOk=false: 401/403 → doRefresh đã clearToken → authGuard redirect /login;
        // lỗi mạng thoáng qua → giữ nguyên state (KHÔNG clear), authGuard retry khi điều hướng.
        return;
      }

      // Case 1b: Token hợp lệ nhưng thiếu user data + hết refresh -> corrupt
      if (hasToken && !hasCached && !hasRefreshToken) {
        console.warn('[AuthService] Token exists but user missing, clearing all');
        await this.clearAllAuthData().catch((err) => console.error('[AuthService] Clear data failed:', err));
        this.tokenService.clearToken();
        this.route.navigate(['/login'], {queryParams: {reason: 'data-corrupt'}});
        return;
      }

      // Case 2: Token hết hạn, refresh còn hạn -> restore UI, authGuard lo silent refresh
      if (!hasToken && hasRefreshToken && hasCached) {
        if (session) {
          this._cachedAuthVersion = session.authVersion;
          authUser.set(session.user);
          this.rbacService.setPermissionsFromStrings(session.permissions);
        } else {
          authUser.set(lsCached);
          this.rbacService.restorePermissions();
          this.storageService.removeStorage(STORAGE_KEY_SELECT.AuthUser); // migrate: xóa key cũ
        }
        return;
      }

      // Case 3: Hết cả hai token -> clear và redirect
      if (!hasToken && !hasRefreshToken && hasCached) {
        await this.clearAllAuthData().catch((err) => console.error('[AuthService] Clear data failed:', err));
        authUser.set(null);
        const currentUrl = this.route.url;
        if (!currentUrl.startsWith('/login')) {
          this.route.navigate(['/login'], {queryParams: {reason: 'token-expired'}});
        }
        return;
      }

      // Case 4: Token hợp lệ + có data -> restore bình thường
      if (hasToken && hasCached) {
        if (session) {
          this._cachedAuthVersion = session.authVersion;
          authUser.set(session.user);
          this.rbacService.setPermissionsFromStrings(session.permissions);
        } else {
          authUser.set(lsCached);
          this.rbacService.restorePermissions();
          this.storageService.removeStorage(STORAGE_KEY_SELECT.AuthUser); // migrate: xóa key cũ
        }
      }
    } finally {
      // Signal cho authGuard biết rằng auth restoration đã hoàn tất (kể cả khi redirect /login).
      // Guards có thể đang đợi signal này trước khi kiểm tra permissions.
      authInitialized.set(true);
    }
  }

  /**
   * Re-fetch /api/auth/me và cập nhật IndexedDB + signals.
   * Gọi sau silent token refresh (permissions/branch/company có thể đã thay đổi)
   * hoặc khi X-User-Auth-Version header lệch (profile đổi).
   * Fire-and-forget safe — lỗi được swallow, không crash app.
   */
  public async fetchAndSyncMe(): Promise<void> {
    if (this._fetchMeInFlight) return;
    this._fetchMeInFlight = true;
    try {
      const body = await firstValueFrom(this.apiService.get(`${this.path}/me`));
      if (body?.user) {
        authUser.set(body.user);
        if (body.permission) {
          this.rbacService.setPermissions(body.permission);
        }
        // setupAuthSync effect sẽ tự save lên IndexedDB
      }
    } catch (err) {
      console.error('[AuthService] fetchAndSyncMe failed:', err);
    } finally {
      this._fetchMeInFlight = false;
    }
  }

  private handleStorageChange = (e: StorageEvent) => {
    // Khi tab khác login (auth_user_id set lần đầu) → reload để populate signals + IndexedDB
    if (e.key === 'auth_user_id' && !e.oldValue && e.newValue) {
      window.location.reload();
    }

    // Khi tab khác logout (auth_user_id bị xóa) → reload để clear và redirect
    if (e.key === 'auth_user_id' && e.oldValue && !e.newValue) {
      window.location.reload();
    }
  };

  private setupStorageListener(): void {
    window.addEventListener('storage', this.handleStorageChange);
  }

  private setupRefreshTokenListener(): void {
    window.addEventListener('refreshTokenExpired', this.refreshTokenHandler);
  }


}
