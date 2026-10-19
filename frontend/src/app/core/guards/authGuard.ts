import { inject } from '@angular/core';
import {
  CanActivateChildFn,
  CanActivateFn,
  CanMatchFn,
  Router,
  UrlSegment,
  UrlTree,
} from '@angular/router';
import { toObservable } from '@angular/core/rxjs-interop';
import { from, Observable, of } from 'rxjs';
import { catchError, filter, map, shareReplay, switchMap, take } from 'rxjs/operators';

import { authInitialized, authUser } from '@/core/services/auth/auth.state';
import { TokenService } from '@/core/services/auth/token.service';
import { GlobalLoadingService } from '@/share/services/global-loading.service';

const LOADING_KEY = 'auth-guard';
let cachedAuth$: Observable<boolean | UrlTree> | null = null;

function toLogin(router: Router, attemptedUrl?: string): UrlTree {
  const redirect = attemptedUrl && !isSyncingRoute(attemptedUrl) ? attemptedUrl : undefined;
  return router.createUrlTree(
    ['/login'],
    redirect ? { queryParams: { redirect } } : undefined,
  );
}

function isLoginRoute(url?: string): boolean {
  if (!url) return false;
  return url === '/login' || url.startsWith('/login?') || url.startsWith('/login/');
}

// /syncing là trang trung gian sau login (xem syncing.ts) — không phải deep-link hợp lệ.
// Nếu để attemptedUrl = /syncing lọt vào redirect param, login xong sẽ bounce lại /syncing
// một lần nữa trước khi tới đích thật (xem afterLoginSuccess trong login.ts).
function isSyncingRoute(url?: string): boolean {
  if (!url) return false;
  return url === '/syncing' || url.startsWith('/syncing?') || url.startsWith('/syncing/');
}

/** S-4/D-1 — màn đổi mật khẩu bắt buộc (xem `withPasswordChangeGate`). */
export const FORCE_CHANGE_PASSWORD_ROUTE = '/change-password-required';

function isForceChangePasswordRoute(url?: string): boolean {
  if (!url) return false;
  return (
    url === FORCE_CHANGE_PASSWORD_ROUTE ||
    url.startsWith(FORCE_CHANGE_PASSWORD_ROUTE + '?') ||
    url.startsWith(FORCE_CHANGE_PASSWORD_ROUTE + '/')
  );
}

/**
 * S-4/D-1 — user còn cờ `mustChangePassword` (login/`/auth/me`) chỉ được ở màn đổi mật khẩu bắt buộc:
 * mọi route khác (kể cả /syncing) bị đưa về đó. Áp SAU khi ensureAuth$ cho qua (authUser đã restore).
 * Lớp enforce thật ở BE (403 `password_change_required`); đây chỉ để không mở app rồi lỗi hàng loạt.
 */
function withPasswordChangeGate(
  result$: Observable<boolean | UrlTree>,
  router: Router,
  attemptedUrl?: string,
): Observable<boolean | UrlTree> {
  return result$.pipe(
    map((res) =>
      res === true && authUser()?.mustChangePassword && !isForceChangePasswordRoute(attemptedUrl)
        ? router.createUrlTree([FORCE_CHANGE_PASSWORD_ROUTE])
        : res,
    ),
  );
}

function isRefreshTokenExpired(tokenService: TokenService): boolean {
  return tokenService.isRefreshTokenExpired();
}

function ensureAuth$(attemptedUrl?: string) {
  const tokenService = inject(TokenService);
  const router = inject(Router);
  const loading = inject(GlobalLoadingService);

  // console.log('[AuthGuard] ensureAuth$ called, url:', attemptedUrl); // Bỏ log để giảm noise

  // Skip nếu đang ở login route
  if (isLoginRoute(attemptedUrl)) {
    console.log('[AuthGuard] Login route, skipping');
    return of(true);
  }

  // Nếu đang refresh -> dùng cache
  if (cachedAuth$) {
    console.log('[AuthGuard] Using cached auth result');
    return cachedAuth$;
  }

  // Show loading ngay từ đầu
  loading.showLoading(LOADING_KEY);

  // Đợi restoreAuthUser() xong trước khi pass — race: withEnabledBlockingInitialNavigation()
  // chạy navigation song song với APP_INITIALIZER, nên requirePerm có thể chạy trước khi
  // userPermissions được populate. authInitialized = true khi restoreAuthUser() hoàn tất.
  // Tạo sẵn ở đây (trong injection context) vì toObservable không gọi được trong switchMap.
  const waitInitialized$: Observable<true> = authInitialized()
    ? of(true as const)
    : toObservable(authInitialized).pipe(
        filter(Boolean),
        take(1),
        map(() => true as const),
      );

  // Token hợp lệ - cho phép truy cập (sau khi restore xong).
  if (tokenService.hasValidToken) {
    loading.hideLoading(LOADING_KEY);
    return waitInitialized$;
  }

  // Refresh token sống trong HttpOnly cookie — JS không đọc được. Chỉ check session expiry qua timestamp.
  // Nếu cookie vẫn còn (BE set khi login/refresh) thì refreshExpiresAt timestamp ở localStorage cũng còn.
  // Cookie và timestamp đồng bộ vì đều set cùng lúc sau mỗi login/refresh.
  if (isRefreshTokenExpired(tokenService)) {
    console.log('[AuthGuard] Refresh session expired');
    tokenService.clearToken();
    loading.hideLoading(LOADING_KEY);
    return of(toLogin(router, attemptedUrl));
  }

  // Thử refresh token và cache kết quả
  cachedAuth$ = from(tokenService.trySilentRefresh()).pipe(
    switchMap((success) => {
      loading.hideLoading(LOADING_KEY);
      cachedAuth$ = null; // Clear cache sau khi xong
      if (!success) {
        // Refresh session còn hạn = fail do lỗi mạng thoáng qua (BE dev restart bởi air,
        // mất mạng...) — doRefresh chỉ clearToken khi 401/403. KHÔNG logout: cho điều
        // hướng tiếp, interceptor sẽ tự refresh lại khi BE sống dậy (401 → trySilentRefresh).
        if (tokenService.hasValidRefreshSession()) {
          console.warn('[AuthGuard] Refresh failed transiently (network/5xx), keeping session');
          return waitInitialized$;
        }
        console.warn('[AuthGuard] Refresh failed, redirecting to login');
        tokenService.clearToken();
        return of(toLogin(router, attemptedUrl));
      }
      // Cũng phải đợi restoreAuthUser xong: khi F5 với access token hết hạn + IDB rỗng,
      // restoreAuthUser (Case 1a) đang fetch /me sau refresh — pass ngay thì requirePerm
      // thấy userPermissions rỗng → accessdenied giả.
      return waitInitialized$;
    }),
    catchError((err) => {
      console.error('[AuthGuard] Refresh error:', err);
      loading.hideLoading(LOADING_KEY);
      cachedAuth$ = null; // Clear cache khi lỗi
      tokenService.clearToken();
      return of(toLogin(router, attemptedUrl));
    }),
    shareReplay(1), // Cache kết quả cho các subscriber tiếp theo
  );

  return cachedAuth$;
}

export const authGuard: CanActivateFn = (_route, state) => {
  const url = state?.url ?? '/';
  return withPasswordChangeGate(ensureAuth$(url), inject(Router), url);
};

export const authCanMatch: CanMatchFn = (_route, segments: UrlSegment[]) => {
  const attempted = '/' + segments.map((s) => s.path).join('/');
  return withPasswordChangeGate(ensureAuth$(attempted), inject(Router), attempted);
};

export const authChildGuard: CanActivateChildFn = (route, state) => authGuard(route, state);
