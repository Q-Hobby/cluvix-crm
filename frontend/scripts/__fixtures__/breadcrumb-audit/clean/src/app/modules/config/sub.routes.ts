// @ts-nocheck
import {Routes} from '@angular/router';

export default [
  {
    path: 'sms',
    loadComponent: () => import('@/pages/plain').then((c) => c.Plain),
    data: {breadcrumb: 'x.sms'},
  },
  {
    path: 'print-template',
    // tự khai routerLink: không thì kế thừa '/config' của wrapper ⇒ B7
    data: {breadcrumb: 'x.tpl_list', routerLink: '/config/print-template'},
    children: [
      {path: '', loadComponent: () => import('@/pages/plain').then((c) => c.Plain)},
      {
        path: 'create',
        loadComponent: () => import('@/pages/plain').then((c) => c.Plain),
        data: {breadcrumb: 'common.create'},
      },
      {
        // B6 nhưng có trong allowlist của fixture
        path: ':id',
        loadComponent: () => import('@/pages/plain').then((c) => c.Plain),
        data: {breadcrumb: 'x.tpl_detail'},
      },
    ],
  },
] as Routes;
