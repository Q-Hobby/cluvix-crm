// @ts-nocheck
// Fixture breadcrumb-audit (KHÔNG phải code app): cây route cố ý chứa đủ 6 loại lỗi B1–B6.
// Số lượng mong đợi khai ở `selftest()` trong scripts/breadcrumb-audit.mjs: B1=3, B2..B6=1.
import {Routes} from '@angular/router';

import {authGuard} from './app/guards';
import {AppLayout} from './app/layout/app.layout';

export const appRoutes: Routes = [
  {
    path: '',
    canActivate: [authGuard],
    loadComponent: () => import('./app/pages/plain').then((c) => c.Plain),
  },
  {
    path: 'crm',
    component: AppLayout,
    canActivate: [authGuard],
    data: {breadcrumb: 'module.crm'},
    loadChildren: () => import('@/modules/crm/crm.routes'),
  },
  {
    // B2: module không có crumb cấp module
    path: 'hr',
    component: AppLayout,
    canActivate: [authGuard],
    loadChildren: () => import('./app/modules/hr/hr.routes'),
  },
  {
    // B7: module có routerLink ở root (component + loadChildren) ⇒ con kế thừa routerLink
    path: 'zalo',
    component: AppLayout,
    canActivate: [authGuard],
    data: {breadcrumb: 'module.zalo', routerLink: '/zalo/user'},
    loadChildren: () => import('@/modules/zalo/zalo.routes'),
  },
  {
    path: '**',
    loadComponent: () => import('./app/pages/plain').then((c) => c.Plain),
  },
];
