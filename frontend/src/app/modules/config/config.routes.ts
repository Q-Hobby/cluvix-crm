import {Routes} from '@angular/router';

import {ConfigLayout} from '@/modules/config/config-layout';
import {PERMISSION_SELECT} from '@/define';
import {requirePerm} from '@/core/guards/has-perm.guard';

export default [
  {
    path: '',
    component: ConfigLayout,
    canActivate: [
      // config-single-permission: 7 quyền `viewConfig*` cũ gộp về 1 key duy nhất.
      // story-20: bỏ requireAnyPerm 2 quyền phụ của 2 module con đã xoá khỏi Cấu hình — chỉ còn
      // 1 permission duy nhất gate layout.
      requirePerm(PERMISSION_SELECT.permissionManageConfig),
    ],
    data: {
      breadcrumb: 'common.label.system_configuration',
      title: 'common.label.system_configuration',
      routerLink: '/config',
    },
    children: [
      {
        path: '',
        loadComponent: () => import('./config/config').then((c) => c.Config),
        data: {breadcrumb: null},
      },
      {
        path: '',
        loadChildren: () => import('./config-system/config-system.routes'),
        canActivate: [requirePerm(PERMISSION_SELECT.permissionManageConfig)],
        data: {redirectTo: '/auth/accessdenied'},
      },
      {
        path: '',
        loadChildren: () => import('./config-clinic/config-clinic.routes'),
        canActivate: [requirePerm(PERMISSION_SELECT.permissionManageConfig)],
        data: {redirectTo: '/auth/accessdenied'},
      },
      {
        path: '',
        loadChildren: () => import('./config-location/config-location.routes'),
        canActivate: [requirePerm(PERMISSION_SELECT.permissionManageConfig)],
        data: {redirectTo: '/auth/accessdenied'},
      },
      {
        path: '',
        loadChildren: () => import('./config-payment/config-payment.routes'),
        canActivate: [requirePerm(PERMISSION_SELECT.permissionManageConfig)],
        data: {redirectTo: '/auth/accessdenied'},
      },
      {
        path: '',
        loadChildren: () => import('./config-appointment/config-appointment.routes'),
        canActivate: [requirePerm(PERMISSION_SELECT.permissionManageConfig)],
        data: {redirectTo: '/auth/accessdenied'},
      },
      {
        path: 'customize',
        loadChildren: () => import('./config-form/config-form.routes'),
        canActivate: [requirePerm(PERMISSION_SELECT.permissionManageConfig)],
        data: {redirectTo: '/auth/accessdenied'},
      },
      {
        path: '',
        loadChildren: () => import('./config-customer/config-customer.routes'),
        canActivate: [requirePerm(PERMISSION_SELECT.permissionManageConfig)],
        data: {redirectTo: '/auth/accessdenied'},
      },
      {
        path: '',
        loadChildren: () => import('./config-department/config-department.routes'),
        canActivate: [requirePerm(PERMISSION_SELECT.permissionManageConfig)],
        data: {redirectTo: '/auth/accessdenied'},
      },
      {
        path: '',
        loadChildren: () => import('./config-print-template/config-print-template.routes'),
        canActivate: [requirePerm(PERMISSION_SELECT.permissionManageConfig)],
        data: {redirectTo: '/auth/accessdenied'},
      },
    ],
  },
] as Routes;
