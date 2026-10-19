package define

// Appointment statuses
// Giá trị số 1–6 KHÔNG được đổi — nhiều luồng nghiệp vụ (tạo hẹn, check-in,
// order_plan, report) so sánh trực tiếp theo giá trị, không theo nhãn.
const (
	AppointmentStatusNotCheckin = 1
	AppointmentStatusCheckedIn  = 2
	AppointmentStatusCheckedOut = 3
	AppointmentStatusCancel     = 4
	AppointmentStatusAdvise     = 5
	AppointmentStatusInService  = 6
)

// AppointmentStatusLabels maps appointment status id → display label.
var AppointmentStatusLabels = map[int64]string{
	AppointmentStatusNotCheckin: "Chưa đến",
	AppointmentStatusCheckedIn:  "Đã đến",
	AppointmentStatusCheckedOut: "Hoàn thành",
	AppointmentStatusCancel:     "Hủy",
	AppointmentStatusAdvise:     "Đang tư vấn",
	AppointmentStatusInService:  "Đang phục vụ",
}

// Appointment bookType
const (
	BookTypeDatTruoc = 1
	BookTypePhatSinh = 2
	BookTypeDuKien   = 3
)

// BookTypeLabels maps bookType id → display label.
var BookTypeLabels = map[int64]string{
	BookTypeDatTruoc: "Đặt trước",
	BookTypePhatSinh: "Phát sinh",
	BookTypeDuKien:   "Dự kiến",
}

// Appointment is_accepted
const (
	IsAccepted    = 1
	IsNotAccepted = 2
)

// IsAcceptedLabels maps is_accepted id → display label.
var IsAcceptedLabels = map[int64]string{
	IsAccepted:    "Đồng ý",
	IsNotAccepted: "Không đồng ý",
}

// Appointment confirmAppointment
const (
	ConfirmAppointmentNot   = 0
	ConfirmAppointmentOk    = 1
	ConfirmAppointmentNotOk = 2
)

// ConfirmAppointmentLabels maps confirmAppointment id → display label.
var ConfirmAppointmentLabels = map[int64]string{
	ConfirmAppointmentNot:   "Chưa xác nhận",
	ConfirmAppointmentOk:    "Xác nhận đến",
	ConfirmAppointmentNotOk: "Xác nhận không đến",
}

// AppointmentOverviewMaxExecMs — trần thời gian chạy (ms) của câu báo cáo tổng quan lịch hẹn
// (`crm/appointment/handler/overview.go`), gắn qua hint MAX_EXECUTION_TIME. LÝ DO BẮT BUỘC: driver
// go-sql-driver/mysql khi context bị huỷ chỉ ĐÓNG SOCKET (mysqlConn.cancel → cleanup), KHÔNG gửi
// KILL QUERY; MySQL không phát hiện client đã đi khi câu đang gom nhóm ⇒ câu chạy tiếp tới khi xong.
// Đã kiểm thật (TestAppointmentOverview_ContextCancel). Trần này là lưới duy nhất phía server chặn
// reload liên tục chồng query 100% CPU.
const AppointmentOverviewMaxExecMs = 30000
