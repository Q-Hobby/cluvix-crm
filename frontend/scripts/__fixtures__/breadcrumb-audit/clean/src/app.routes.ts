// @ts-nocheck
// Fixture breadcrumb-audit (KHÔNG phải code app): cây route sạch, phủ các ca kế thừa `data` dễ mô phỏng sai.
import {Routes} from '@angular/router';

import {authGuard} from './app/guards';
import {AppLayout} from '@/layout/app.layout';
import {ConfigLayout} from './app/layout/config.layout';
import {printRoutes} from '@/modules/print.routes';

export const appRoutes: Routes = [
  {
    path: '',
    canActivate: [authGuard],
    loadComponent: () => import('./app/pages/plain').then((c) => c.Plain),
  },
  {
    // module khai cấp module ở wrapper '' trong config.routes.ts
    path: 'config',
    component: ConfigLayout,
    canActivate: [authGuard],
    loadChildren: () => import('@/modules/config/config.routes'),
  },
  {
    path: 'crm',
    component: AppLayout,
    canActivate: [authGuard],
    data: {breadcrumb: 'module.crm', preload: true},
    loadChildren: () => import('@/modules/crm/crm.routes'),
  },
  {
    // module không có route '' ⇒ routerLink tới trang đích
    path: 'zalo',
    component: AppLayout,
    canActivate: [authGuard],
    // permission ở root: con path khác rỗng (`perm`) kế thừa qua vế loadChildren (ca a)
    data: {breadcrumb: 'module.zalo', routerLink: '/zalo/user', permission: P.viewZalo},
    loadChildren: () => import('@/modules/zalo/zalo.routes'),
  },
  // spread kiểu emrPrintRoutes: path gốc không nằm trong 15 nhánh ⇒ bỏ qua kiểm lỗi
  ...printRoutes,
  {
    path: '**',
    loadComponent: () => import('./app/pages/plain').then((c) => c.Plain),
  },
];
