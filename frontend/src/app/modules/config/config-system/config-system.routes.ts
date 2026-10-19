import { Routes } from '@angular/router';

export default [
  {
    path: 'company',
    loadComponent: () =>
      import('@/modules/config/config-system/business-profile/business-profile').then((c) => c.BusinessProfile),
    data: { breadcrumb: 'config.business_profile.title' },
  },
  {
    path: 'security-audit',
    loadComponent: () =>
      import('@/modules/config/config-system/security-audit/security-audit').then((c) => c.SecurityAudit),
    data: { breadcrumb: 'config.security_audit.title' },
  },
  {
    path: 'branch',
    loadComponent: () =>
      import('@/modules/config/config-system/branch/list/branch-list').then((c) => c.BranchList),
    data: { breadcrumb: 'common.label.branch' },
  },
  {
    path: 'config_code',
    loadComponent: () =>
      import('@/modules/config/config-system/config-code/list/config-code-list').then((c) => c.ConfigCodeList),
    data: { breadcrumb: 'common.label.identifier_code' },
  },
  {
    path: 'unit',
    loadComponent: () =>
      import('@/modules/config/config-system/unit/list/unit-list').then((c) => c.UnitList),
    data: { breadcrumb: 'common.label.unit' },
  },
  {
    path: 'option',
    loadComponent: () =>
      import('@/modules/config/config-system/option-crm/option-crm').then((c) => c.OptionCrm),
    data: { breadcrumb: 'config.nav.system.task_options' },
  },
  {
    path: 'setting',
    loadComponent: () =>
      import('@/modules/config/config-system/setting/setting').then((c) => c.Setting),
    data: { breadcrumb: 'config.nav.system.setting' },
  },
] as Routes;
