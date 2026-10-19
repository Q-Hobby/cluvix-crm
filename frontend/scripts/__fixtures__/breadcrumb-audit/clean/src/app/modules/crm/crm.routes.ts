// @ts-nocheck
import {Routes} from '@angular/router';

import {P} from '@/define';
import {permissionGuard, requirePerm} from '@/guards';

export default [
  {
    path: '',
    loadComponent: () => import('@/pages/plain').then((c) => c.Plain),
    data: {breadcrumb: null},
  },
  {
    // cha KHÔNG component + loadChildren ⇒ con '' kế thừa breadcrumb ⇒ khử trùng
    path: 'customer',
    canActivate: [requirePerm(P.viewCustomer)],
    loadChildren: () => import('./customer.routes'),
    data: {breadcrumb: 'x.customer_list', preload: true},
  },
  {
    // cha CÓ component + children inline (không loadChildren): con 'mid' (path khác rỗng, không data) VẪN kế thừa
    // (runtime, Đính chính lần 2) ⇒ lá 'leaf' nhận routerLink '/crm/group' (mục cuối, không render link)
    path: 'group',
    loadComponent: () => import('@/pages/plain').then((c) => c.Plain),
    data: {breadcrumb: 'x.group', routerLink: '/crm/group'},
    children: [
      {path: '', loadComponent: () => import('@/pages/plain').then((c) => c.Plain), data: {breadcrumb: null}},
      {
        path: 'mid',
        children: [
          {path: 'leaf', loadComponent: () => import('@/pages/plain').then((c) => c.Plain), data: {breadcrumb: 'x.exam'}},
        ],
      },
    ],
  },
  {
    // cha component + children inline, KHÔNG loadChildren; con component thiếu `permission` ⇒ vẫn nhận của cha
    path: 'pgroup',
    loadComponent: () => import('@/pages/plain').then((c) => c.Plain),
    data: {breadcrumb: 'x.income', permission: P.viewGroup},
    children: [
      {path: '', loadComponent: () => import('@/pages/plain').then((c) => c.Plain), data: {breadcrumb: null}},
      {
        path: 'child',
        canActivate: [permissionGuard],
        loadComponent: () => import('@/pages/plain').then((c) => c.Plain),
        data: {breadcrumb: 'x.exam'},
      },
    ],
  },
  {
    // khuôn C3': danh sách logic không phải tổ tiên URL
    path: 'exam',
    data: {breadcrumb: 'x.reception', routerLink: '/crm/customer'},
    children: [
      {
        path: ':id',
        loadComponent: () => import('@/pages/with-label').then((c) => c.WithLabel),
        data: {breadcrumb: 'x.exam'},
      },
    ],
  },
  // ── F1: bất biến dump khi dời permissionGuard (guard đọc data của CHÍNH route khai guard) ──
  {
    // baseline phẳng: guard + permission trên lá
    path: 'stock-flat',
    canActivate: [permissionGuard],
    loadComponent: () => import('@/pages/plain').then((c) => c.Plain),
    data: {breadcrumb: 'x.income', permission: P.viewStock},
  },
  {
    // C3 hợp lệ: guard + permission cùng ở cha không component (lá kế thừa)
    path: 'stock-c3',
    canActivate: [permissionGuard],
    data: {breadcrumb: 'x.income', permission: P.viewStock},
    children: [
      {path: '', loadComponent: () => import('@/pages/plain').then((c) => c.Plain)},
      {path: ':id', loadComponent: () => import('@/pages/with-label').then((c) => c.WithLabel), data: {breadcrumb: 'x.exam'}},
    ],
  },
  {
    // hợp lệ: permission ở cha không component, guard giữ ở từng lá (lá kế thừa permission)
    path: 'stock-leafguard',
    data: {breadcrumb: 'x.income', permission: P.viewStock},
    children: [
      {path: '', canActivate: [permissionGuard], loadComponent: () => import('@/pages/plain').then((c) => c.Plain)},
      {
        path: ':id',
        canActivate: [permissionGuard],
        loadComponent: () => import('@/pages/with-label').then((c) => c.WithLabel),
        data: {breadcrumb: 'x.exam'},
      },
    ],
  },
  {
    // SAI: guard dời lên cha nhưng permission để ở lá ⇒ runtime guard đọc data rỗng ⇒ mất kiểm quyền
    path: 'stock-wrong',
    canActivate: [permissionGuard],
    data: {breadcrumb: 'x.income'},
    children: [
      {path: '', loadComponent: () => import('@/pages/plain').then((c) => c.Plain), data: {permission: P.viewStock}},
      {
        path: ':id',
        loadComponent: () => import('@/pages/with-label').then((c) => c.WithLabel),
        data: {breadcrumb: 'x.exam', permission: P.viewStock},
      },
    ],
  },
  {
    path: 'income',
    canActivate: [permissionGuard],
    loadComponent: () => import('@/pages/plain').then((c) => c.Plain),
    data: {breadcrumb: 'x.income', permission: P.viewIncome},
  },
] as Routes;
