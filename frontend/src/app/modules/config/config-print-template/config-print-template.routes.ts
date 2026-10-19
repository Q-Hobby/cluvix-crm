import { Routes } from '@angular/router';

export default [
  {
    path: 'print-template',
    data: { breadcrumb: 'config.nav.panel.medical_form', routerLink: '/config/print-template' },
    children: [
      {
        path: '',
        loadComponent: () =>
          import('./print-template-list/print-template-list').then((c) => c.PrintTemplateList),
        data: { title: 'config.nav.panel.medical_form' },
      },
      {
        path: 'create',
        loadComponent: () =>
          import('./print-template-page/print-template-page').then((c) => c.PrintTemplatePage),
        data: { breadcrumb: 'common.action.create', title: 'config.nav.panel.medical_form' },
      },
      {
        path: ':id',
        loadComponent: () =>
          import('./print-template-page/print-template-page').then((c) => c.PrintTemplatePage),
        data: { breadcrumb: 'common.action.update', title: 'config.nav.panel.medical_form' },
      },
    ],
  },
] as Routes;
