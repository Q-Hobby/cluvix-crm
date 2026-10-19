import {inject} from '@angular/core';
import {Router, Routes} from '@angular/router';

import {authGuard} from '@/core/guards/authGuard';
import {loginGuard} from '@/core/guards/loginGuard';
import {AppLayout} from '@/layout/components/app.layout';
import {PrintLayout} from '@/layout/print/print.layout';

export const appRoutes: Routes = [
  {
    path: 'auth',
    canActivate: [authGuard],
    loadChildren: () => import('@/modules/auth/auth.routes'),
  },
  {
    path: 'config',
    component: AppLayout,
    canActivate: [authGuard],
    data: {preload: true},
    loadChildren: () => import('@/modules/config/config.routes'),
  },
  {
    path: 'hr',
    component: AppLayout,
    canActivate: [authGuard],
    data: {preload: true, breadcrumb: 'common.label.hr'},
    loadChildren: () => import('@/modules/hr/hr.routes'),
  },
  {
    path: 'log',
    component: AppLayout,
    canActivate: [authGuard],
    data: {breadcrumb: 'common.label.history', routerLink: '/log/export'},
    loadChildren: () => import('@/modules/history/log.routes'),
  },
  {
    path: 'print',
    component: PrintLayout,
    canActivate: [authGuard],
    loadChildren: () => import('@/prints/prints.routes'),
  },
  {
    path: 'login',
    canActivate: [loginGuard],
    loadComponent: () => import('@/modules/auth/login/login').then((c) => c.Login),
  },
  {
    // Ép setup 2FA — user đã login nhưng policy bắt buộc chưa enroll (authGuard, không layout).
    path: '2fa-setup',
    canActivate: [authGuard],
    loadComponent: () => import('@/modules/auth/twofa-setup/twofa-setup').then((c) => c.TwofaSetup),
  },
  {
    // S-4/D-1 — ép đổi mật khẩu (user.mustChangePassword): authGuard đưa mọi route khác về đây
    // (không layout), interceptor bắt 403 `password_change_required`.
    path: 'change-password-required',
    canActivate: [authGuard],
    loadComponent: () =>
      import('@/modules/auth/force-change-password/force-change-password').then((c) => c.ForceChangePassword),
  },
  {
    // Trang chờ đồng bộ IndexedDB sau login/2FA, trước khi vào layout chính (authGuard, không layout).
    path: 'syncing',
    canActivate: [authGuard],
    loadComponent: () => import('@/modules/auth/syncing/syncing').then((c) => c.Syncing),
  },
  {
    // Link/bookmark cũ `/crm/...` (trước 2026-10-19) → bỏ tiền tố, giữ query + fragment.
    path: 'crm',
    children: [
      {
        path: '**',
        // Trả UrlTree (không phải string) — redirect dạng string từ hàm làm rơi query string.
        redirectTo: ({url, queryParams, fragment}) =>
          inject(Router).createUrlTree(['/', ...url.map((s) => s.path)], {queryParams, fragment: fragment ?? undefined}),
      },
    ],
  },
  {
    // CRM thuần cho SME: module CRM nằm thẳng ở gốc (2026-10-19 bỏ tầng `/crm`) — '/' là dashboard,
    // `/customer`, `/appointment`, `/payment`... Đặt SAU các route tường minh: path '' khớp mọi URL,
    // con không khớp thì router quay lui xuống '**'.
    path: '',
    component: AppLayout,
    canActivate: [authGuard],
    data: {preload: true},
    loadChildren: () => import('@/modules/crm/crm.routes'),
  },
  {
    path: '**',
    canActivate: [authGuard],
    loadComponent: () => import('@/modules/auth/notfound/notfound').then((c) => c.Notfound),
    data: {title: '404 - Not Found'},
  },
];
