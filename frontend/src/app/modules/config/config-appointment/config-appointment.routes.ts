import { Routes } from '@angular/router';

export default [
  {
    path: 'appointment-reason',
    loadComponent: () =>
      import('@/modules/config/config-appointment/appointment-reason/list/appointment-reason-list').then(
        (c) => c.AppointmentReasonList,
      ),
    data: { breadcrumb: 'config.nav.appointment.rejection_reason' },
  },
  {
    path: 'appointment-type',
    loadComponent: () =>
      import('@/modules/config/config-appointment/appointment-type/list/appointment-type-list').then(
        (c) => c.AppointmentTypeList,
      ),
    data: {
      breadcrumb: 'common.label.appointment_type',
    },
  },
  {
    path: 'appointment-is-accepted',
    loadComponent: () =>
      import('@/modules/config/config-appointment/appointment-is-accepted/list/appointment-is-accepted-list').then(
        (c) => c.AppointmentIsAcceptedList,
      ),
    data: { breadcrumb: 'config.nav.appointment.consultation_status' },
  },
  {
    path: 'appointment-not-booked-reasons',
    loadComponent: () =>
      import('@/modules/config/config-appointment/appointment-not-booked-reasons/list/appointment-not-booked-reasons-list').then(
        (c) => c.AppointmentNotBookedReasonsList,
      ),
    data: { breadcrumb: 'common.label.not_booked_reason' },
  },
] as Routes;
