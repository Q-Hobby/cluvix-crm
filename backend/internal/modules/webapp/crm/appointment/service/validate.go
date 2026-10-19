package service

import (
	appointmentDao "erpCluvix/internal/modules/webapp/crm/appointment/dao"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/validation"
)

// validateAppointmentCreate / Update — port từ be/common/models/Appointment.php::rules()
// Required: appointment_type_id, branch_id, appointment_date, customer_id.
// Cross-field: checkout_date >= checkin_date.
// Enum: status in AppointmentStatus.
func validateAppointmentCreate(a *appointmentDao.Appointment) error {
	v := validation.New()
	v.Required(a.AppointmentTypeId, "Loại lịch hẹn")
	v.Required(a.BranchId, "Chi nhánh")
	v.Required(a.AppointmentDate, "Ngày hẹn")
	v.Required(a.CustomerId, "Khách hàng")
	checkAppointmentCommon(v, a)
	if v.HasErrors() {
		return v
	}
	return nil
}

func validateAppointmentUpdate(a *appointmentDao.Appointment) error {
	v := validation.New()
	checkAppointmentCommon(v, a)
	if v.HasErrors() {
		return v
	}
	return nil
}

func checkAppointmentCommon(v *validation.Errors, a *appointmentDao.Appointment) {
	if a.Status != nil {
		v.In(*a.Status, []int{
			define.AppointmentStatusNotCheckin,
			define.AppointmentStatusCheckedIn,
			define.AppointmentStatusCheckedOut,
			define.AppointmentStatusCancel,
			define.AppointmentStatusAdvise,
			define.AppointmentStatusInService,
		}, "Trạng thái")
	}
	if a.CheckinDate != nil && a.CheckoutDate != nil && *a.CheckoutDate < *a.CheckinDate {
		v.Add("Giờ hoàn thành không thể trước giờ đến")
	}
}
