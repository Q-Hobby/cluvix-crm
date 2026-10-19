import { inject } from '@angular/core';
import { ActivatedRouteSnapshot, CanActivateFn, Router, UrlTree } from '@angular/router';

import { TokenService } from '@/core/services/auth/token.service';

/** Chỉ cho redirect path nội bộ — chống open-redirect (//evil.com, javascript:, http://...). */
function safeInternalRedirect(value: unknown): string {
  if (typeof value !== 'string') return '/';
  // Phải bắt đầu bằng "/" và không phải protocol-relative "//..."
  if (!value.startsWith('/') || value.startsWith('//')) return '/';
  return value;
}

export const loginGuard: CanActivateFn = (route: ActivatedRouteSnapshot): boolean | UrlTree => {
  const tokenService = inject(TokenService);
  const router = inject(Router);

  // Redirect nếu có valid token HOẶC refresh session còn hạn (access token có thể expired
  // nhưng authGuard trên protected route sẽ tự silent refresh).
  const isAuthenticated = tokenService.hasValidToken || tokenService.hasValidRefreshSession();
  if (isAuthenticated) {
    const target = safeInternalRedirect(route.queryParams['redirect']);
    // parseUrl xử lý query params/fragments có sẵn trong target
    return router.parseUrl(target);
  }

  return true;
};
