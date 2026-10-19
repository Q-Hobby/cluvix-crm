import { Routes } from '@angular/router';

export default [
  {
    path: '',
    loadComponent: () =>
      import('./contact-list/contact-list').then((c) => c.ContactList),
    data: {
      title: 'common.label.customer_management',
    },
  },
  {
    path: 'detail/:id',
    loadComponent: () =>
      import('@/modules/crm/customer/customer-detail/customer-detail').then((c) => c.CustomerDetail),
    data: { breadcrumb: 'crm.customer.detail', title: 'crm.customer.detail', preload: true },
  },
] as Routes;
