// auth.interceptor.ts
import {
  HttpErrorResponse,
  HttpEvent,
  HttpHandlerFn,
  HttpInterceptorFn,
  HttpRequest,
  HttpResponse,
} from '@angular/common/http';
import { inject } from '@angular/core';
import { Observable, from, throwError } from 'rxjs';
import { catchError, switchMap, tap } from 'rxjs/operators';
import { TokenService } from '@/core/services/auth/token.service';
import { AuthService } from '@/core/services/auth/auth.service';
import { ReminderAlertService } from '@/core/services/reminder-alert.service';

/** Skip token attach — login/refresh-token gửi credentials riêng, không cần Bearer */
function isNoTokenEndpoint(url: string): boolean {
  return /\/auth\/(login|refresh-token)\b/.test(url);
}

/** Đọc locale hiện tại từ localStorage 'lng' để gửi Accept-Language cho BE.
 * BE dùng header này để translate sentinel errors / response messages. */
function getCurrentLocale(): string {
  try {
    const raw = localStorage.getItem('lng');
    if (raw) {
      const parsed = JSON.parse(raw);
      if (typeof parsed === 'string') return parsed;
    }
  } catch {
    // localStorage không khả dụng hoặc JSON parse fail → fallback default
  }
  return 'vi-VN';
}

/** Skip 401 retry loop — không retry/refresh cho toàn bộ auth endpoints kể cả logout */
function isAuthEndpoint(url: string): boolean {
  return /\/auth\/(login|logout|refresh-token)\b/.test(url);
}

export const authInterceptor: HttpInterceptorFn = (
  req: HttpRequest<unknown>,
  next: HttpHandlerFn,
): Observable<HttpEvent<unknown>> => {
  const tokenService = inject(TokenService);
  const authService = inject(AuthService);
  const reminderAlert = inject(ReminderAlertService);

  // Skip auth for OPTIONS — vẫn gửi Accept-Language cho login/refresh để translate sentinel errors
  if (req.method === 'OPTIONS') {
    return next(req);
  }

  const acceptLanguage = getCurrentLocale();

  // No-token endpoints (login, refresh-token) — chỉ gửi Accept-Language, không Bearer
  if (isNoTokenEndpoint(req.url)) {
    return next(req.clone({ setHeaders: { 'Accept-Language': acceptLanguage } }));
  }

  // Chỉ attach token, không tự động refresh (để AuthGuard lo)
  const accessToken = tokenService.getAccessToken();
  const headers: Record<string, string> = { 'Accept-Language': acceptLanguage };
  if (accessToken) headers['Authorization'] = `Bearer ${accessToken}`;
  const modifiedReq = req.clone({ setHeaders: headers });

  return next(modifiedReq).pipe(
    tap((evt) => {
      if (evt instanceof HttpResponse) {
        const reminderCount = evt.headers.get('Cluvix-Reminder');
        if (reminderCount) {
          const count = parseInt(reminderCount, 10);
          if (!isNaN(count)) {
            reminderAlert.setCount(count);
          }
        }
      }
    }),
    catchError((err: any) => {
      const status = (err as HttpErrorResponse)?.status;

      if (status === 0) {
        return throwError(() => err);
      }

      const is401or419 = (status === 401 || status === 419) && !isAuthEndpoint(req.url);
      if (is401or419) {
        // Nếu retry cũng 401 → KHÔNG retry nữa.
        // Phân biệt: session-expired (token thực sự hết) vs endpoint-specific 401
        // (BE trả 401 nhầm cho permission/scope/resource issue-2604).
        // Chỉ logout nếu FE xác nhận token đã expire — nếu không, propagate error
        // để component xử lý (không invalidate session toàn cục).
        if (req.headers.has('X-Retry-After-Refresh')) {
          if (!tokenService.isTokenExpired()) {
            console.warn(
              '[AuthInterceptor] Retry 401 nhưng access token còn hạn — endpoint-specific 401 (permission/scope/BE), KHÔNG logout',
              { url: req.url, status },
            );
            return throwError(() => err);
          }
          console.warn('[AuthInterceptor] Retry still 401 + token expired, logging out', { url: req.url });
          tokenService.clearToken();
          setTimeout(() => window.dispatchEvent(new CustomEvent('refreshTokenExpired')), 0);
          return throwError(() => err);
        }

        // Check xem token trong localStorage đã khác token gửi đi không
        // (auto-refresh đã chạy xong → token mới đã có)
        const currentToken = tokenService.getAccessToken();
        const sentToken = modifiedReq.headers.get('Authorization')?.replace('Bearer ', '');

        if (currentToken && currentToken !== sentToken) {
          // Token đã được refresh → retry ngay với token mới, không cần gọi refresh
          console.log('[AuthInterceptor] Token already refreshed, retrying with new token');
          const retryReq = req.clone({
            setHeaders: {
              Authorization: `Bearer ${currentToken}`,
              'Accept-Language': acceptLanguage,
              'X-Retry-After-Refresh': '1',
            },
          });
          return next(retryReq);
        }

        // Token chưa được refresh → thử refresh rồi retry
        return from(tokenService.trySilentRefresh()).pipe(
          switchMap((refreshed) => {
            if (refreshed) {
              // Re-fetch user data sau refresh — permission/branch/company có thể đã đổi
              // (admin revoke token khi update role/status/branch/company).
              // Fire-and-forget: không block retry request, update async trong background.
              void authService.fetchAndSyncMe();

              const newToken = tokenService.getAccessToken();
              const retryReq = req.clone({
                setHeaders: {
                  Authorization: `Bearer ${newToken}`,
                  'Accept-Language': acceptLanguage,
                  'X-Retry-After-Refresh': '1',
                },
              });
              return next(retryReq);
            }
            // Refresh session còn hạn = refresh fail do lỗi mạng/5xx thoáng qua (BE dev
            // restart bởi air) — doRefresh chỉ clearToken khi 401/403. KHÔNG logout,
            // propagate error để component xử lý; lần request sau sẽ refresh lại được.
            if (tokenService.hasValidRefreshSession()) {
              console.warn('[AuthInterceptor] 401/419 + refresh failed transiently, NOT logging out', {
                url: req.url,
                status,
              });
              return throwError(() => err);
            }
            console.warn('[AuthInterceptor] 401/419 + refresh failed, logging out', {
              url: req.url,
              status,
            });
            tokenService.clearToken();
            setTimeout(() => window.dispatchEvent(new CustomEvent('refreshTokenExpired')), 0);
            return throwError(() => err);
          }),
        );
      }

      return throwError(() => err);
    }),
  );
};
