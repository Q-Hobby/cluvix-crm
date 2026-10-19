// @ts-nocheck
import {Routes} from '@angular/router';

export default [
  {
    path: 'user',
    loadComponent: () => import('@/pages/plain').then((c) => c.Plain),
    data: {breadcrumb: 'x.list'},
  },
  {
    // B7 (ca b): mục không-cuối, KHÔNG tự khai routerLink ⇒ href kế thừa '/zalo/user' của root zalo
    path: 'camp',
    data: {breadcrumb: 'x.detail'},
    children: [
      {path: '', loadComponent: () => import('@/pages/plain').then((c) => c.Plain)},
      {path: 'create', loadComponent: () => import('@/pages/plain').then((c) => c.Plain), data: {breadcrumb: 'x.hr'}},
    ],
  },
] as Routes;
