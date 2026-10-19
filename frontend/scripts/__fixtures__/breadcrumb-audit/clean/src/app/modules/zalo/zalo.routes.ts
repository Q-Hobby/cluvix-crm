// @ts-nocheck
import {Routes} from '@angular/router';

import {permissionGuard} from '@/guards';
import {ZaloUserPage} from './zalo-user';

const zaloRoutes: Routes = [
  {path: 'user', component: ZaloUserPage, data: {breadcrumb: 'x.zalo_user'}},
  // ca (a): con path khác rỗng của root zalo (component + loadChildren), không tự khai breadcrumb/permission
  // ⇒ runtime kế thừa data root ⇒ breadcrumb bị khử trùng, permissionGuard đọc permission của root
  {path: 'perm', canActivate: [permissionGuard], loadComponent: () => import('@/pages/plain').then((c) => c.Plain)},
  {
    // ca (c): mục không-cuối TỰ KHAI routerLink ⇒ không B7
    path: 'camp',
    data: {breadcrumb: 'x.camp', routerLink: '/zalo/camp'},
    children: [
      {path: '', loadComponent: () => import('@/pages/plain').then((c) => c.Plain)},
      {path: 'create', loadComponent: () => import('@/pages/plain').then((c) => c.Plain), data: {breadcrumb: 'common.create'}},
    ],
  },
];

export default zaloRoutes;
