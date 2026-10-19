// response-handler.interceptor.ts
import {
  HttpContextToken,
  HttpErrorResponse,
  HttpEvent,
  HttpHandlerFn,
  HttpInterceptorFn,
  HttpRequest,
} from '@angular/common/http';
import { inject } from '@angular/core';
import { Router } from '@angular/router';
import { TranslateService } from '@ngx-translate/core';
import { from, Observable, of, throwError, timer } from 'rxjs';
import { concatMap, map, retry, tap } from 'rxjs/operators';

import { AuthService } from '@/core/services/auth/auth.service';
import { TokenService } from '@/core/services/auth/token.service';
import { UserAuthSessionService } from '@/core/services/auth/user-auth-session.service';
import { NotificationService } from '@/share/services/notification.service';
import { HeaderSyncService } from '@/core/services/header-sync.service';
import { authUser } from '@/core/services/auth/auth.state';
import { FORCE_CHANGE_PASSWORD_ROUTE } from '@/core/guards/authGuard';
import { CONFIG_FILE_SELECT } from '@/define';

// Flag để tạm disable sync sau login
let syncDisabledUntil = 0;

/** Tạm disable sync trong khoảng thời gian (ms) */
export function disableSyncTemporarily(durationMs: number = 5000) {
  syncDisabledUntil = Date.now() + durationMs;
  console.log(`Sync disabled for ${durationMs}ms`);
}

/** Skip automatic error notification */
export const SKIP_ERROR_NOTIFICATION = new HttpContextToken<boolean>(() => false);

/**
 * Không điều hướng cả trang sang `/auth/notfound` khi API trả 404.
 *
 * Mặc định mọi 404 của `/api/*` bị coi là "trang hiện tại sai địa chỉ" → đá người dùng ra khỏi
 * màn đang làm việc. Điều đó chỉ đúng với request NUÔI CẢ TRANG. Với request PHỤ của một màn đang
 * mở (panel chi tiết, popup, lazy block), record biến mất là lỗi CỤC BỘ: đá cả trang là làm mất
 * luôn danh sách + bộ lọc người dùng đang có, để đổi lấy một thông tin lẽ ra chỉ cần báo trong
 * panel. Caller đặt token này PHẢI tự xử lý nhánh error (hiện thông báo tại chỗ).
 */
export const SKIP_GLOBAL_404 = new HttpContextToken<boolean>(() => false);

/**
 * story-14 (epic notification-debt-cleanup) — cùng khuôn `SKIP_GLOBAL_404`, cho 403.
 *
 * R-4 (2026-10-18): interceptor KHÔNG còn điều hướng sang `/auth/accessdenied` cho 403 nữa (mặc định
 * nay là toast) — cờ này giờ chỉ còn tác dụng TẮT luôn toast, cho caller tự báo tại chỗ.
 *
 * (Lịch sử) Không điều hướng cả trang sang `/auth/accessdenied` khi API trả 403 KHÔNG kèm `error_user_msg`.
 * Lý do y hệt 404: mặc định mọi 403 bị coi là "người này không được vào màn đang mở" — chỉ đúng với
 * request NUÔI CẢ TRANG. Với request PHỤ (panel chi tiết, dialog timeline, GET theo id của deep-link
 * từ chuông thông báo) thì mất quyền trên MỘT bản ghi là lỗi CỤC BỘ: đá cả trang là làm mất danh
 * sách + bộ lọc người dùng đang có. Ca chạm thật: `GetInstance` trả `response.Forbidden` khi uỷ
 * quyền hết hạn/đổi người duyệt ⇒ bấm thông báo approval cũ văng khỏi inbox.
 *
 * Cờ TẮT = hành vi cũ nguyên vẹn (mặc định `false`). Caller đặt token này PHẢI tự xử lý nhánh error
 * (báo tại chỗ / đóng dialog im lặng). KHÔNG ảnh hưởng nhánh 403 đặc biệt phía trên
 * (`2fa_setup_required`) — server chặn MỌI API nên bắt buộc phải điều hướng, cờ cục bộ không được
 * phép tắt. (story-42: BE bỏ hẳn gate `company_active_required` — story-58 — nhánh tương ứng đã xoá.)
 */
export const SKIP_GLOBAL_403 = new HttpContextToken<boolean>(() => false);

export const responseHandlerInterceptor: HttpInterceptorFn = (
  request: HttpRequest<unknown>,
  next: HttpHandlerFn,
): Observable<HttpEvent<unknown>> => {
  const router = inject(Router);
  const authService = inject(AuthService);
  const tokenService = inject(TokenService);
  const noti = inject(NotificationService);
  const headerSync = inject(HeaderSyncService);
  const userAuthSession = inject(UserAuthSessionService);
  const translate = inject(TranslateService);

  return next(request).pipe(
    // PWA mở từ shortcut màn hình chính (standalone launch, khác tab browser
    // vốn network stack đã "warm") → request GET đầu tiên lúc cold-start có
    // thể đua với OS bring-up network, fail status 0 dù máy có mạng thật
    // (navigator.onLine vẫn true). Retry âm thầm 1 lần trước khi báo lỗi.
    // CHỈ GET — POST/PUT/DELETE không retry để tránh double-submit nếu
    // request thực ra đã tới server mà response bị rớt giữa chừng.
    retry({
      count: 1,
      delay: (error) => {
        const isConnectionError =
          request.method === 'GET' &&
          error instanceof HttpErrorResponse &&
          error.status === 0 &&
          navigator.onLine !== false &&
          error.error?.name !== 'AbortError' &&
          error.error?.name !== 'CanceledError';
        return isConnectionError ? timer(700) : throwError(() => error);
      },
    }),
    concatMap((event: any) => {
      if (event?.type !== 4 || !event?.headers) return of(event);
      const url = request.url.toLowerCase();
      const skipSync =
        url.includes('indexeddb/sync') ||
        url.includes('/assets/') ||
        url.includes('/auth/login');
      if (skipSync || Date.now() <= syncDisabledUntil) return of(event);
      // QUAN TRỌNG: Luôn check headers, kể cả cached response — chờ sync xong trước khi trả response
      return from(
        headerSync.checkVersionFromHeaders(event.headers)
          .then(async () => {
            // Per-user auth version check — chỉ user đang login mới sync khi header lệch
            const userAuthVer = event.headers.get('X-User-Auth-Version');
            const currentUserId = authUser()?.id;
            if (userAuthVer && currentUserId) {
              const stale = await userAuthSession.isStale(userAuthVer, currentUserId);
              if (stale) {
                // Cập nhật version trước khi fetch để tránh trigger lặp nếu /me response chậm
                const parts = userAuthVer.split(':');
                const newVer = parseInt(parts[1], 10);
                if (!isNaN(newVer)) {
                  authService._cachedAuthVersion = newVer; // sync in-memory ngay
                  await userAuthSession.updateAuthVersion(currentUserId, newVer);
                }
                void authService.fetchAndSyncMe();
              }
            }
          })
          .catch((err: any) => {
            console.error('Header sync error:', err);
          }),
      ).pipe(map(() => event));
    }),
    tap({
      next: (event: any) => {
        // Response đã qua sync
      },

      error: (err: any) => {
        // Skip nếu request có context token
        if (request.context.get(SKIP_ERROR_NOTIFICATION)) {
          return;
        }

        // Nếu server trả Blob (HTML/text), đọc sang text rồi JSON (async)
        if (err instanceof HttpErrorResponse && err.error instanceof Blob) {
          const reader = new FileReader();
          reader.onload = () => {
            const text = reader.result as string;
            let parsed: any = text;
            try {
              parsed = JSON.parse(text);
            } catch {}
            const fixed = new HttpErrorResponse({
              error: parsed,
              headers: err.headers,
              status: err.status,
              statusText: err.statusText,
              url: err.url ?? undefined,
            });
            handleAuthError(
              fixed,
              router,
              authService,
              tokenService,
              noti,
              translate,
              request.url, // pass original URL
              request.context.get(SKIP_GLOBAL_404),
              request.context.get(SKIP_GLOBAL_403),
            );
          };
          reader.readAsText(err.error);
          return;
        }

        if (err instanceof HttpErrorResponse) {
          handleAuthError(
            err,
            router,
            authService,
            tokenService,
            noti,
            translate,
            request.url, // pass original URL
            request.context.get(SKIP_GLOBAL_404),
            request.context.get(SKIP_GLOBAL_403),
          );
        }
      },
    }),
  );
};

function handleAuthError(
  err: HttpErrorResponse,
  router: Router,
  _authService: AuthService,
  tokenService: TokenService,
  noti: NotificationService,
  translate: TranslateService,
  originalReqUrl?: string,
  skipGlobal404 = false,
  skipGlobal403 = false,
): void {
  const t = (key: string) => translate.instant(key);
  switch (err.status) {
    case 204: {
      noti.info(t('common.message.no_data'), stringifyPayload(err.error));
      break;
    }

    case 401: {
      // 401 đã được xử lý bởi auth.interceptor
      // Chỉ hiển thị notification nếu cần
      const atLogin = router.routerState.snapshot.url.startsWith('/login');
      if (atLogin) {
        noti.error('Unauthorized', t('common.message.auth_invalid'));
      }
      break;
    }

    case 403: {
      // Policy bắt buộc 2FA nhưng chưa enroll → ép vào trang setup (server chặn mọi API khác).
      if (err?.error && typeof err.error === 'object' && err.error?.code === '2fa_setup_required') {
        if (!router.routerState.snapshot.url.startsWith('/2fa-setup')) {
          void router.navigate(['/2fa-setup']);
        }
        break;
      }
      // S-4/D-1 — cờ bắt buộc đổi mật khẩu (server chặn mọi API nghiệp vụ) → màn đổi mật khẩu bắt buộc.
      if (err?.error && typeof err.error === 'object' && err.error?.code === 'password_change_required') {
        if (!router.routerState.snapshot.url.startsWith(FORCE_CHANGE_PASSWORD_ROUTE)) {
          void router.navigate([FORCE_CHANGE_PASSWORD_ROUTE]);
        }
        break;
      }
      const msg403 = err?.error && typeof err.error === 'object' ? err.error?.error_user_msg : null;
      if (msg403) {
        noti.error(t('common.message.no_permission'), msg403);
        break;
      }
      // story-14: request PHỤ của màn đang mở → caller tự báo lỗi tại chỗ, không đá cả trang đi.
      // Đặt SAU nhánh `2fa_setup_required` và sau `error_user_msg` để cờ cục bộ không tắt được
      // điều hướng bắt buộc lẫn thông báo server đã soạn sẵn. (story-42: nhánh `company_active_required`
      // đã xoá — BE không còn trả code này, xem story-58.)
      if (skipGlobal403) {
        console.warn('403 Forbidden (xử lý cục bộ, không điều hướng):', err.url || originalReqUrl || '');
        break;
      }
      // R-4 (2026-10-18): 403 của REQUEST DỮ LIỆU ⇒ chỉ toast, KHÔNG đá cả trang sang /auth/accessdenied.
      // Chặn ở cấp màn hình là việc của route guard (permissionGuard/hasPermGuard tự điều hướng
      // accessdenied). BE nay enforce cả GET (S-9) ⇒ một khối phụ thiếu quyền (vd ghi chú trong chi tiết
      // khách) mà đá cả trang là mất luôn màn người dùng ĐƯỢC quyền xem; component tự hiện rỗng.
      noti.error(
        t('common.message.no_permission'),
        (err?.error && typeof err.error === 'object' ? err.error?.message : null) ||
          t('common.message.no_permission'),
      );
      break;
    }

    case 404: {
      const url = err.url || originalReqUrl || '';
      const isApiRequest = /\/(api|crm)\//i.test(url); // chỉnh theo API
      if (isApiRequest) {
        // Request phụ của màn đang mở → caller tự báo lỗi tại chỗ, không đá cả trang đi.
        if (skipGlobal404) {
          console.warn('404 Not Found (xử lý cục bộ, không điều hướng):', url);
          break;
        }
        void router.navigate(['/auth/notfound']);
      } else {
        console.warn('404 Not Found (Asset/File):', url);
      }
      break;
    }

    case 409: {
      const msg409 = err.error?.message ?? t('common.message.data_conflict');
      noti.warn(t('common.message.data_conflict'), msg409);
      break;
    }

    case 410: {
      void router.navigate(['/auth/error']);
      break;
    }

    case 413: {
      const cl = safeNum(err.error?.contentLength);
      const max = safeNum(err.error?.post_max_size);
      const maxSizeMB = (CONFIG_FILE_SELECT.CONFIG_FILE_MAX_SIZE / 1024 / 1024).toFixed(0);
      const msg =
        cl && max
          ? `Tổng dung lượng ${prettyBytes(cl)} vượt giới hạn ${prettyBytes(max)}. Vui lòng giảm dung lượng hoặc tải từng ảnh.`
          : err.error?.message ||
            `Dung lượng file vượt quá giới hạn máy chủ. Vui lòng chọn file nhỏ hơn ${maxSizeMB}MB.`;
      noti.warn(t('common.message.file_too_large'), msg);
      break;
    }

    case 422: {
      const e = err.error;
      if (typeof e === 'string') {
        noti.warn(t('common.message.auth_error'), e);
      } else if (typeof e === 'object') {
        if (e.message) {
          noti.warn(t('common.message.auth_error'), e.message);
        } else {
          // Xử lý validation errors: { "password": ["Sai thông tin..."] }
          const messages: string[] = [];
          Object.values(e).forEach((value: any) => {
            if (Array.isArray(value)) {
              messages.push(...value);
            } else {
              messages.push(String(value));
            }
          });
          // Hiển thị message đầu tiên
          if (messages.length > 0) {
            noti.warn(t('common.message.auth_error'), messages[0]);
          } else {
            noti.warn(t('common.message.auth_error'), t('common.message.invalid_data'));
          }
        }
      } else {
        noti.warn(t('common.message.auth_error'), t('common.message.invalid_data'));
      }
      break;
    }

    case 400: {
      const payload = err.error;
      const title =
        payload && typeof payload === 'object' && payload.name ? payload.name : 'Bad Request';
      const msg =
        payload && typeof payload === 'object' && payload.message
          ? payload.message
          : typeof payload === 'string'
            ? payload
            : t('common.message.request_invalid');
      noti.error(title, msg);
      break;
    }

    case 429: {
      const payload = err.error;
      const msg =
        payload && typeof payload === 'object' && payload.message
          ? payload.message
          : t('common.message.too_many_selected');
      noti.error(t('common.message.retry_later'), msg);
      break;
    }

    case 500: {
      const e = err.error;
      if (typeof e === 'string') {
        noti.error('Internal Server Error', e);
      } else if (typeof e === 'object') {
        switch (e.name) {
          case 'Exception':
          case 'PHP Warning':
          case 'Database Exception':
          case 'Internal Server Error':
            noti.error('Internal Server Error', e.message || t('common.message.unknown_error'));
            break;
          case 'PHP Fatal Error':
            noti.error('Internal Server Error', t('common.message.unknown_error'));
            break;
          default:
            Object.values(e).forEach((value: any) => {
              Array.isArray(value)
                ? value.forEach((msg: string) => noti.error('Internal Server Error', msg))
                : noti.error('Internal Server Error', String(value));
            });
        }
      } else {
        noti.error('Internal Server Error', t('common.message.unknown_error'));
      }
      break;
    }

    case 0: {
      const aborted =
        err.error?.name === 'AbortError' ||
        err.error?.name === 'CanceledError' ||
        /canceled/i.test(err.message || '');
      if (aborted) break;

      const isOffline = navigator.onLine === false;
      if (isOffline) {
        noti.error(t('common.message.offline'), t('common.message.offline_detail'));
        break;
      }

      const url = err.url || originalReqUrl || '';
      const sameOrigin = (() => {
        try {
          const u = new URL(url, location.origin);
          return u.origin === location.origin;
        } catch {
          return true;
        }
      })();

      const looksLikeCors =
        !sameOrigin && /Failed to fetch|CORS|preflight/i.test(err.message || '');
      if (looksLikeCors) {
        noti.error('CORS', t('common.message.cors_blocked_detail'));
        break;
      }

      const blockedByExt = /ERR_BLOCKED_BY_CLIENT/i.test(err.message || '');
      if (blockedByExt) {
        noti.warn(t('common.message.extension_blocked'), t('common.message.extension_blocked_detail'));
        break;
      }

      const sslIssue = /SSL|CERT|certificate/i.test(err.message || '');
      if (sslIssue) {
        noti.error(t('common.message.ssl_error'), t('common.message.ssl_error_detail'));
        break;
      }

      noti.error(t('common.message.server_unreachable'), t('common.message.server_unreachable_detail'));
      break;
    }

    default: {
      const title = err.error?.name || `${t('common.message.error')} ${err.status || ''}`.trim();
      const msg = err.error?.message || err.message || t('common.message.error');
      noti.error(title, msg);
      break;
    }
  }
}

/* ===== Helpers ===== */
function stringifyPayload(e: any): string {
  if (e == null) return '';
  if (typeof e === 'string') return e;
  if (typeof e === 'object') {
    if (e.message) return e.message;
    try {
      return JSON.stringify(e);
    } catch {
      return String(e);
    }
  }
  return String(e);
}

function safeNum(n: any): number | null {
  const x = Number(n);
  return Number.isFinite(x) ? x : null;
}

function prettyBytes(bytes?: number | null): string {
  if (!bytes && bytes !== 0) return '';
  const units = ['B', 'KB', 'MB', 'GB'];
  let i = 0,
    b = bytes!;
  while (b >= 1024 && i < units.length - 1) {
    b /= 1024;
    i++;
  }
  return `${b.toFixed(1)} ${units[i]}`;
}
