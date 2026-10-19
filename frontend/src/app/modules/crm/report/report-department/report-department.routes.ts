import { Routes } from '@angular/router';

export default [
  {
    path: 'revenue',
    loadComponent: () => import('./revenue/revenue').then((c) => c.Revenue),
    data: { breadcrumb: 'crm.report.department.consulting' },
  },
  {
    path: 'telesales-online',
    loadComponent: () =>
      import('./telesales-online/telesales-online').then((c) => c.TelesalesOnline),
    data: { breadcrumb: 'crm.report.department.telesales_online' },
  },
  {
    path: 'effective-advise',
    loadComponent: () =>
      import('./effective-advise/effective-advise').then((c) => c.EffectiveAdvise),
    data: { breadcrumb: 'crm.report.department.appointment_effective' },
  },
] as Routes;
