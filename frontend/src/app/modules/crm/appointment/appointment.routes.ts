import {Routes} from '@angular/router';

export default [
  // {
  //   path: 'full-calendar',
  //   loadComponent: () =>
  //     import('@/apps/appointment/full-calendar/full-calendar').then((c) => c.FullCalendar),
  //   data: { breadcrumb: 'PAGE_TITLE.LIST_APPOINTMENT', title: 'PAGE_TITLE.LIST_APPOINTMENT' },
  // },
  {
    path: '',
    loadComponent: () =>
      import('@/modules/crm/appointment/list/in-menu/appointment-in-menu').then(
        (c) => c.AppointmentInMenu,
      ),
    data: {breadcrumb: 'layout.app_menu.appointment_list', title: 'layout.app_menu.appointment_list'},
  },
  {
    path: 'appointment-schedule',
    loadComponent: () => import('./list/schedule/schedule').then((c) => c.Schedule),
    data: {
      breadcrumb: 'crm.appointment.page.by_day',
      title: 'crm.appointment.page.by_day',
    },
  },
  {
    path: 'appointment-schedule-byhour',
    loadComponent: () =>
      import('./list/schedule-byhour/schedule-byhour').then((c) => c.ScheduleByhour),
    data: {
      breadcrumb: 'crm.appointment.page.by_hour',
      title: 'crm.appointment.page.by_hour',
    },
  },
] as Routes;
