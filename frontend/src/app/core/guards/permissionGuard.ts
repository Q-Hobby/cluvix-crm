import { CanActivateFn, Router } from '@angular/router';
import { inject } from '@angular/core';
import { hasPermission } from '@/core/services/auth/auth.selectors';

export const permissionGuard: CanActivateFn = (route) => {
  const perm = route.data?.['permission'] as string;
  if (!perm || hasPermission(perm)) return true;
  return inject(Router).createUrlTree(['/auth/accessdenied']);
};
