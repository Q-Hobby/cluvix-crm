import { inject, Service, signal } from '@angular/core';
import { HttpClient, HttpXhrBackend } from '@angular/common/http';
import { firstValueFrom } from 'rxjs';
import { environment } from '@env/environment';
import { SecureStorageService } from '@/core/services/storage/secure-storage.service';
import { getDeviceFingerprint } from './fingerprint';

@Service()
export class TokenService {
  private refreshInProgress: Promise<boolean> | null = null;
  private autoRefreshTimer?: any;
  private storageListener?: (e: StorageEvent) => void;

  // In-memory signal cache — populated by initialize() before app routing
  private _accessToken = signal<string | null>(null);

  // HttpClient riêng, bypass interceptor → tránh circular dependency
  private refreshHttp = new HttpClient(inject(HttpXhrBackend));
  private refreshUrl = environment.apiUrl.replace(/\/+$/, '') + '/auth/refresh-token';
  private secure = inject(SecureStorageService);

  constructor() {
    this.setupStorageSync();
  }

  /** Called by APP_INITIALIZER — decrypts stored access token into memory before routing starts */
  async initialize(): Promise<void> {
    const at = await this.decryptFromStorage('access_token');
    this._accessToken.set(at);
    // Clean up legacy refresh_token in localStorage (Cấp 2 stores it in HttpOnly cookie now)
    localStorage.removeItem('refresh_token');
    this.scheduleAutoRefresh();
  }

  public get hasValidToken(): boolean {
    const token = this.getAccessToken();
    if (!token) return false;
    const expiresAt = localStorage.getItem('token_expires_at');
    if (expiresAt) return Date.now() < parseInt(expiresAt, 10) * 1000;
    return !this.isTokenExpired(token);
  }

  public get isRefreshing(): boolean {
    return this.refreshInProgress !== null;
  }

  // ========== Token Management ==========
  public async setTokens(
    accessToken: string,
    expiresAt?: number,
    refreshExpiresAt?: number,
  ): Promise<void> {
    // Update in-memory signal immediately so sync callers (interceptor, guards) see new tokens
    this._accessToken.set(accessToken);
    // Default access token expiry to 15 min from now if BE doesn't provide.
    // Dùng `||` (không phải `??`) để fallback cả khi BE trả 0 — defensive vì 0
    // là Unix epoch (1970), khiến hasValidToken/isRefreshTokenExpired báo expired.
    const accessExpiresAt = expiresAt || Math.floor(Date.now() / 1000) + 15 * 60;
    localStorage.setItem('token_expires_at', accessExpiresAt.toString());
    // Default refresh expiry to 7 days from now if BE doesn't provide.
    const refreshExpAt = refreshExpiresAt || Math.floor(Date.now() / 1000) + 7 * 24 * 60 * 60;
    localStorage.setItem('refresh_token_expires_at', refreshExpAt.toString());
    this.scheduleAutoRefresh();
    // Encrypt and persist access token — refresh token sống trong HttpOnly cookie do BE set
    const encAt = await this.secure.encrypt(accessToken);
    localStorage.setItem('access_token', encAt);
  }

  public getAccessToken(): string | null {
    return this._accessToken();
  }

  /**
   * Refresh token sống trong HttpOnly cookie — JS không đọc được.
   * Caller chỉ check session valid qua `!isRefreshTokenExpired()`.
   */
  public hasValidRefreshSession(): boolean {
    return !this.isRefreshTokenExpired();
  }

  public clearToken(): void {
    console.warn('[TokenService] clearToken() called');
    localStorage.removeItem('access_token');
    localStorage.removeItem('refresh_token'); // legacy cleanup
    localStorage.removeItem('token_expires_at');
    localStorage.removeItem('refresh_token_expires_at');
    // Xoá luôn auth_user + permissions để tránh state inconsistent:
    // tokens đã gone nhưng cached user còn → restoreAuthUser Case 3 fire
    // → navigate /login?reason=token-expired đè redirect param của authGuard.
    localStorage.removeItem('auth_user');
    localStorage.removeItem('config_permission');
    this._accessToken.set(null);
    if (this.autoRefreshTimer) {
      clearTimeout(this.autoRefreshTimer);
      this.autoRefreshTimer = undefined;
    }
  }

  // ========== Token Validation ==========
  public isTokenExpired(token?: string): boolean {
    const accessToken = token || this.getAccessToken();
    if (!accessToken) return true;

    const expiresAt = localStorage.getItem('token_expires_at');
    if (expiresAt) {
      return Date.now() >= parseInt(expiresAt, 10) * 1000;
    }

    // Fallback: decode JWT
    try {
      const payload = JSON.parse(atob(accessToken.split('.')[1]));
      return Date.now() >= payload.exp * 1000;
    } catch {
      return true;
    }
  }

  /**
   * Refresh token expiry tracked qua localStorage timestamp (BE trả `refreshExpiresAt`).
   * FE không đọc được token value — chỉ biết khi nào hết hạn.
   */
  public isRefreshTokenExpired(): boolean {
    const refreshExpiresAt = localStorage.getItem('refresh_token_expires_at');
    if (!refreshExpiresAt) return true;
    return Date.now() >= parseInt(refreshExpiresAt, 10) * 1000;
  }

  // ========== Refresh Logic ==========
  public async ensureValidToken(): Promise<boolean> {
    if (!this.getAccessToken()) return false;
    if (this.isTokenExpired()) {
      return await this.trySilentRefresh();
    }
    return true;
  }

  public async trySilentRefresh(): Promise<boolean> {
    if (this.refreshInProgress) {
      console.log('[TokenService] Refresh already in progress, waiting...');
      return await this.refreshInProgress;
    }

    if (this.isRefreshTokenExpired()) {
      console.warn('[TokenService] Refresh session expired');
      this.clearToken();
      this.dispatchLogoutEvent();
      return false;
    }

    this.refreshInProgress = this.doRefresh();
    try {
      return await this.refreshInProgress;
    } finally {
      this.refreshInProgress = null;
    }
  }

  private waitForLockRelease(): Promise<boolean> {
    return new Promise((resolve) => {
      if (!localStorage.getItem('refresh_lock')) {
        resolve(!!this._accessToken() && !this.isTokenExpired());
        return;
      }

      const timeout = setTimeout(() => {
        window.removeEventListener('storage', handler);
        resolve(!!this._accessToken() && !this.isTokenExpired());
      }, 10000);

      const handler = (e: StorageEvent) => {
        if (e.key !== 'refresh_lock') return;
        if (!e.newValue) {
          clearTimeout(timeout);
          window.removeEventListener('storage', handler);
          setTimeout(() => {
            resolve(!!this._accessToken() && !this.isTokenExpired());
          }, 50);
        }
      };

      window.addEventListener('storage', handler);
    });
  }

  private async doRefresh(): Promise<boolean> {
    const existingLock = localStorage.getItem('refresh_lock');
    const existingLockTime = existingLock ? parseInt(existingLock.split(':')[0], 10) : 0;
    if (existingLock && Date.now() - existingLockTime < 10000) {
      console.log('[TokenService] Another tab is refreshing, waiting...');
      return this.waitForLockRelease();
    }

    const tabId = Math.random().toString(36).slice(2);
    localStorage.setItem('refresh_lock', `${Date.now()}:${tabId}`);

    await new Promise(r => setTimeout(r, 50));

    const currentLock = localStorage.getItem('refresh_lock');
    if (!currentLock?.endsWith(`:${tabId}`)) {
      console.log('[TokenService] Lost lock race, waiting for other tab...');
      return this.waitForLockRelease();
    }

    try {
      // refresh_token đến từ HttpOnly cookie; body chỉ mang device_fingerprint
      // để BE verify device binding (mode log/enforce). Lỗi tính fp → gửi rỗng,
      // BE coi như client không gửi (không chặn ở mode enforce với token legacy).
      let fp = '';
      try {
        fp = await getDeviceFingerprint();
      } catch {
        /* fingerprint không khả dụng → bỏ qua */
      }

      // Lỗi mạng (status 0, vd BE dev đang restart bởi air) / 5xx là transient →
      // retry với backoff thay vì fail ngay. Tổng delay ~6s phải < ngưỡng stale lock
      // 10s để tab khác không cướp lock giữa chừng (mỗi lần retry re-stamp lock).
      const retryDelays = [1000, 2000, 3000];
      for (let attempt = 0; ; attempt++) {
        try {
          const body = await firstValueFrom(
            this.refreshHttp.post<any>(this.refreshUrl, { device_fingerprint: fp }, { withCredentials: true }),
          );

          if (body?.ok && body?.accessToken) {
            await this.setTokens(body.accessToken, body.expiresAt, body.refreshExpiresAt);
            return true;
          }

          console.warn('[TokenService] Refresh response invalid, clearing token');
          this.clearToken();
          return false;
        } catch (err: any) {
          const status = (err as any)?.status;
          const transient = status === 0 || (typeof status === 'number' && status >= 500);
          const delay = retryDelays[attempt];
          if (transient && delay !== undefined) {
            console.warn(`[TokenService] Refresh transient error (status ${status}), retrying in ${delay}ms...`);
            localStorage.setItem('refresh_lock', `${Date.now()}:${tabId}`);
            await new Promise(r => setTimeout(r, delay));
            continue;
          }
          console.error('[TokenService] Refresh error:', status || err?.message || err);
          // Chỉ clear session khi token thực sự invalid (401/403).
          // Lỗi mạng / 5xx là transient — không xóa refresh_token_expires_at vì session
          // vẫn valid ở Redis, clearToken ở đây gây "Refresh session expired" giả.
          if (status === 401 || status === 403) {
            this.clearToken();
          }
          return false;
        }
      }
    } finally {
      const lock = localStorage.getItem('refresh_lock');
      if (lock?.endsWith(`:${tabId}`)) {
        localStorage.removeItem('refresh_lock');
      }
    }
  }

  // ========== Auto Refresh ==========
  private scheduleAutoRefresh(): void {
    if (this.autoRefreshTimer) {
      clearTimeout(this.autoRefreshTimer);
    }

    const expiresAt = localStorage.getItem('token_expires_at');
    if (!expiresAt) return;

    const expiresTime = parseInt(expiresAt, 10) * 1000;
    const now = Date.now();
    const timeUntilExpiry = expiresTime - now;
    const refreshBeforeExpiry = 120 * 1000;
    const timeUntilRefresh = timeUntilExpiry - refreshBeforeExpiry;

    if (timeUntilRefresh > 0) {
      this.autoRefreshTimer = setTimeout(() => {
        console.log('[TokenService] Auto-refresh triggered');
        this.trySilentRefresh();
      }, timeUntilRefresh);
    } else if (timeUntilExpiry > 0) {
      console.log('[TokenService] Token expiring soon, refreshing now');
      this.trySilentRefresh();
    }
  }

  private dispatchLogoutEvent(): void {
    setTimeout(() => window.dispatchEvent(new CustomEvent('refreshTokenExpired')), 0);
  }

  // ========== Cross-Tab Sync ==========
  private setupStorageSync(): void {
    this.storageListener = async (e: StorageEvent) => {
      if (e.key === 'access_token') {
        if (e.newValue) {
          // Tab khác refresh thành công → decrypt và cập nhật signal
          const decrypted = await this.decryptFromStorage('access_token');
          this._accessToken.set(decrypted);
          this.scheduleAutoRefresh();
        } else if (e.oldValue && !e.newValue && !localStorage.getItem('refresh_lock')) {
          this._accessToken.set(null);
          this.clearToken();
          this.dispatchLogoutEvent();
        }
      }

      if (e.key === 'refresh_lock' && e.newValue) {
        if (this.autoRefreshTimer) {
          clearTimeout(this.autoRefreshTimer);
          this.autoRefreshTimer = undefined;
        }
      }

      if (e.key === 'refresh_lock' && !e.newValue) {
        this.scheduleAutoRefresh();
      }
    };
    window.addEventListener('storage', this.storageListener);
  }

  private async decryptFromStorage(key: 'access_token'): Promise<string | null> {
    const stored = localStorage.getItem(key);
    if (!stored) return null;
    if (this.secure.isCiphertext(stored)) {
      return this.secure.decrypt(stored);
    }
    // Plain JWT (legacy from before Cấp 1) — migrate it to encrypted form
    const encrypted = await this.secure.encrypt(stored);
    localStorage.setItem(key, encrypted);
    return stored;
  }
}
