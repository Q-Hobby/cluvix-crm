import { HttpContext, provideHttpClient, withInterceptors } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { Router } from '@angular/router';
import { provideTranslateService } from '@ngx-translate/core';
import { HttpClient } from '@angular/common/http';
import { afterEach, describe, expect, it, vi } from 'vitest';

// ⚠ Import NÀY phải đứng TRƯỚC interceptor (cùng lý do đã ghi ở `reminder-in-workspace.deeplink.spec.ts`):
// vòng `auth.service` ⇄ `base-api.service` chỉ nổ dưới thứ tự nạp ESM của vitest.
import { AuthService } from '@/core/services/auth/auth.service';
import { TokenService } from '@/core/services/auth/token.service';
import { UserAuthSessionService } from '@/core/services/auth/user-auth-session.service';
import { HeaderSyncService } from '@/core/services/header-sync.service';
import { NotificationService } from '@/share/services/notification.service';
import {
  responseHandlerInterceptor,
  SKIP_GLOBAL_403,
} from '@/core/interceptors/response-handler.interceptor';
import { FORCE_CHANGE_PASSWORD_ROUTE } from '@/core/guards/authGuard';

void AuthService;

/**
 * story-14 (epic notification-debt-cleanup) AC2/AC4 — cửa "xử lý 403 tại chỗ".
 *
 * Điều đang khoá: cờ `SKIP_GLOBAL_403` BẬT ⇒ interceptor KHÔNG `router.navigate` đi đâu cả; cờ TẮT
 * (mặc định) ⇒ R-4 (2026-10-18): toast lỗi, KHÔNG còn đá sang `/auth/accessdenied` (chặn cấp màn hình là
 * việc của route guard). Interceptor là hạ tầng TOÀN APP, đổi mặc định là đổi hành vi của mọi màn.
 *
 * Mutation check (AC5): bỏ nhánh `if (skipGlobal403)` trong `case 403` của
 * `response-handler.interceptor.ts` ⇒ ca "cờ BẬT" phải ĐỎ.
 */
function setup(routerUrl = '/care/customer') {
  const navigate = vi.fn();
  const noti = { error: vi.fn(), warn: vi.fn(), info: vi.fn(), warnMsg: vi.fn() };
  TestBed.configureTestingModule({
    providers: [
      provideHttpClient(withInterceptors([responseHandlerInterceptor])),
      provideHttpClientTesting(),
      provideTranslateService(),
      { provide: AuthService, useValue: { fetchAndSyncMe: vi.fn(), _cachedAuthVersion: 0 } },
      { provide: TokenService, useValue: {} },
      { provide: UserAuthSessionService, useValue: { isStale: () => Promise.resolve(false) } },
      {
        provide: HeaderSyncService,
        useValue: { checkVersionFromHeaders: () => Promise.resolve() },
      },
      { provide: NotificationService, useValue: noti },
      {
        provide: Router,
        useValue: { navigate, routerState: { snapshot: { url: routerUrl } } },
      },
    ],
  });
  return { navigate, noti, http: TestBed.inject(HttpTestingController), client: TestBed.inject(HttpClient) };
}

/** Bắn 1 GET rồi trả về `status` cho trước; nuốt lỗi để test không đỏ vì unhandled error. */
function flushWith(
  client: HttpClient,
  http: HttpTestingController,
  status: number,
  body: unknown,
  context?: HttpContext,
) {
  client.get('/api/crm/customer-care/9', context ? { context } : {}).subscribe({
    next: () => {},
    error: () => {},
  });
  http.expectOne((r) => r.url === '/api/crm/customer-care/9').flush(body as any, {
    status,
    statusText: 'Err',
  });
}

describe('responseHandlerInterceptor — SKIP_GLOBAL_403 (story-14)', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('AC2: cờ BẬT + 403 ⇒ KHÔNG điều hướng (caller tự báo tại chỗ)', () => {
    const { navigate, http, client } = setup();

    flushWith(client, http, 403, { message: 'forbidden' }, new HttpContext().set(SKIP_GLOBAL_403, true));

    expect(navigate).not.toHaveBeenCalled();
  });

  it('R-4: cờ TẮT (mặc định) + 403 dữ liệu ⇒ toast lỗi, KHÔNG đá cả trang sang /auth/accessdenied', () => {
    const { navigate, http, client, noti } = setup();

    flushWith(client, http, 403, { message: 'forbidden' });

    expect(navigate).not.toHaveBeenCalled();
    expect(noti.error).toHaveBeenCalledTimes(1);
    expect(noti.error.mock.calls[0][1]).toBe('forbidden');
  });

  it('R-4: cờ BẬT + 403 ⇒ không toast (caller tự báo tại chỗ)', () => {
    const { http, client, noti } = setup();

    flushWith(client, http, 403, { message: 'forbidden' }, new HttpContext().set(SKIP_GLOBAL_403, true));

    expect(noti.error).not.toHaveBeenCalled();
  });

  it('R-4: 403 `password_change_required` (A3) VẪN điều hướng sang màn đổi mật khẩu bắt buộc', () => {
    const { navigate, http, client, noti } = setup();

    flushWith(client, http, 403, { code: 'password_change_required', message: 'x' });

    expect(navigate).toHaveBeenCalledWith([FORCE_CHANGE_PASSWORD_ROUTE]);
    expect(noti.error).not.toHaveBeenCalled();
  });

  it('R-4: 401 KHÔNG điều hướng/toast ở interceptor này (auth.interceptor lo refresh)', () => {
    const { navigate, http, client, noti } = setup();

    flushWith(client, http, 401, { message: 'unauthorized' });

    expect(navigate).not.toHaveBeenCalled();
    expect(noti.error).not.toHaveBeenCalled();
  });

  it('AC2: cờ BẬT KHÔNG được tắt nhánh bắt buộc `2fa_setup_required`', () => {
    const { navigate, http, client } = setup();

    flushWith(
      client,
      http,
      403,
      { code: '2fa_setup_required' },
      new HttpContext().set(SKIP_GLOBAL_403, true),
    );

    expect(navigate).toHaveBeenCalledWith(['/2fa-setup']);
  });

  it('AC2: cờ BẬT KHÔNG đụng 404 (vẫn đá sang /auth/notfound như cũ)', () => {
    const { navigate, http, client } = setup();

    flushWith(
      client,
      http,
      404,
      { message: 'not found' },
      new HttpContext().set(SKIP_GLOBAL_403, true),
    );

    expect(navigate).toHaveBeenCalledWith(['/auth/notfound']);
  });

  it('AC2: cờ BẬT KHÔNG đụng 410 (vẫn đá sang /auth/error như cũ)', () => {
    const { navigate, http, client } = setup();

    flushWith(client, http, 410, { message: 'gone' }, new HttpContext().set(SKIP_GLOBAL_403, true));

    expect(navigate).toHaveBeenCalledWith(['/auth/error']);
  });
});
