// @ts-nocheck
import {Routes} from '@angular/router';

export const printRoutes: Routes = [
  {
    path: 'crm/print/:id',
    loadComponent: () => import('../pages/plain').then((c) => c.Plain),
    data: {breadcrumb: 'Raw ngoài layout breadcrumb'},
  },
];
