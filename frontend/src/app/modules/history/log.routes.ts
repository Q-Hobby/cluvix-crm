import { Routes } from '@angular/router';
import { requirePerm } from '@/core/guards/has-perm.guard';
import { PERMISSION_SELECT } from '@/define';

export default [
  {
    path: '',
    canActivate: [requirePerm(PERMISSION_SELECT.permissionViewHistory)],
    children: [
      {
        path: 'export',
        loadComponent: () => import('@/modules/history/export/export-history').then((c) => c.ExportHistory),
        data: { breadcrumb: 'layout.log_menu.export', title: 'layout.log_menu.export' },
      },
      {
        path: 'login',
        loadComponent: () => import('@/modules/history/login/login-history').then((c) => c.LoginHistory),
        data: { breadcrumb: 'layout.log_menu.login', title: 'layout.log_menu.login' },
      },
      {
        path: 'customer',
        loadComponent: () => import('@/modules/history/customer/customer-log').then((c) => c.CustomerLog),
        data: { breadcrumb: 'layout.log_menu.customer', title: 'layout.log_menu.customer' },
      },
      {
        path: 'appointment',
        loadComponent: () => import('@/modules/history/appointment/appointment-log').then((c) => c.AppointmentLog),
        data: { breadcrumb: 'layout.log_menu.appointment', title: 'layout.log_menu.appointment' },
      },
      {
        path: 'payment',
        loadComponent: () => import('@/modules/history/payment/payment-log').then((c) => c.PaymentLog),
        data: { breadcrumb: 'common.label.payment_history', title: 'common.label.payment_history' },
      },
      {
        path: 'order-service-used',
        loadComponent: () => import('@/modules/history/order-service-used/order-service-used-log').then((c) => c.OrderServiceUsedLog),
        data: { breadcrumb: 'layout.log_menu.order_service_used', title: 'layout.log_menu.order_service_used' },
      },
      {
        path: 'user',
        loadComponent: () => import('@/modules/history/user/user-log').then((c) => c.UserLog),
        data: { breadcrumb: 'layout.log_menu.user', title: 'layout.log_menu.user' },
      },
    ],
  },
] as Routes;
