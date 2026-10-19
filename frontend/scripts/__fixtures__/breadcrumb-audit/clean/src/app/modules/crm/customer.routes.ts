// @ts-nocheck
import {Routes} from '@angular/router';

export default [
  {path: '', loadComponent: () => import('@/pages/plain').then((c) => c.Plain)},
  {
    path: 'detail/:id',
    loadComponent: () => import('@/pages/with-label').then((c) => c.WithLabel),
    data: {breadcrumb: 'x.customer_detail'},
  },
] as Routes;
