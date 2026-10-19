// @ts-nocheck
import {Routes} from '@angular/router';

export default [
  {
    path: '',
    loadComponent: () => import('../../pages/plain').then((c) => c.Plain),
    data: {breadcrumb: 'x.hr'},
  },
] as Routes;
