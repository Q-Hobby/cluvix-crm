// @ts-nocheck
import {Routes} from '@angular/router';

import {P} from '@/define';
import {requirePerm} from '@/guards';
import {ConfigShell} from './config-shell';

export default [
  {
    path: '',
    component: ConfigShell,
    canActivate: [
      requirePerm(
        // comment trong guard không được lọt vào dump-map
        P.manage,
      ),
    ],
    data: {breadcrumb: 'module.config', routerLink: '/config'},
    children: [
      {
        path: '',
        loadComponent: () => import('@/pages/plain').then((c) => c.Plain),
        data: {breadcrumb: null},
      },
      {
        path: '',
        loadChildren: () => import('./sub.routes'),
        data: {redirectTo: '/auth/accessdenied'},
      },
    ],
  },
] as Routes;
