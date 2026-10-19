import { CanActivateFn, Router } from '@angular/router';
import { inject } from '@angular/core';
import { hasAnyPermission, hasPermission } from '@/core/services/auth/auth.selectors';

/**
 * Guard factory: yêu cầu user có 1 permission cụ thể.
 * Super role (admin/develop) bypass tự động qua hasPermission().
 */
export function requirePerm(permName: string): CanActivateFn {
  return (route) => {
    if (hasPermission(permName)) return true;
    const redirect = (route.data?.['redirectTo'] as string) || '/auth/accessdenied';
    return inject(Router).createUrlTree([redirect]);
  };
}

/**
 * Guard factory: yêu cầu user có ÍT NHẤT 1 trong list permission.
 * Dùng cho module-level route (ví dụ /config có 7 sub-permission, chỉ cần 1).
 */
export function requireAnyPerm(...permNames: string[]): CanActivateFn {
  return (route) => {
    if (hasAnyPermission(...permNames)) return true;
    const redirect = (route.data?.['redirectTo'] as string) || '/auth/accessdenied';
    return inject(Router).createUrlTree([redirect]);
  };
}
