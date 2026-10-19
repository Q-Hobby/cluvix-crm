import { Routes } from '@angular/router';

export default [
  {
    path: 'form-customer',
    data: { breadcrumb: 'config.nav.custom_form.customer' },
    loadComponent: () => import('./form-customer/form-customer').then((m) => m.FormCustomer),
  },
  {
    path: 'form-lead',
    data: { breadcrumb: 'config.nav.custom_form.lead' },
    loadComponent: () => import('./form-lead/form-lead').then((m) => m.FormLead),
  },
] as Routes;
