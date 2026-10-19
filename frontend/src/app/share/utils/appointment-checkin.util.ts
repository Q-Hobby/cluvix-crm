import { format } from 'date-fns';

/**
 * Xác định appointment có được phép bấm "Khách đến" (checkin) hay không.
 * Dùng chung cho các list hiển thị appointment (AppointmentInMenu, ScheduleBydoctorHourList...).
 * appointmentDate (ms, đã normalize) dùng cho cả sameDay lẫn pastNotCheckout —
 * KHÔNG so sánh trực tiếp model.appointment_date (có thể là raw unix-second) với `today`.
 */
export const isCheckinAble = (
  model: any,
  today: Date,
  cancelStatus: number,
  checkoutStatus: number,
): boolean => {
  const appointmentDate = typeof model.appointment_date === 'number'
    ? model.appointment_date * 1000
    : model.appointment_date?.getTime?.() ?? 0;
  const sameDay = format(today, 'yyyy-MM-dd') === format(appointmentDate, 'yyyy-MM-dd');
  const pastNotCheckout = today.getTime() >= appointmentDate && model.status !== checkoutStatus;
  return (sameDay || pastNotCheckout) && model.status !== cancelStatus;
};
