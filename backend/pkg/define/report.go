package define

// ReportMaxRangeSec — trần độ rộng khoảng ngày của báo cáo thống kê CRM.
//
// 366 ngày (1 năm, cộng 1 ngày cho năm nhuận) tính theo giây, dùng cho các endpoint report
// aggregate quét dải ngày trên bảng nghiệp vụ lớn. Lý do có trần, đo 2026-09-09 trên bench
// 1.090.355 lịch hẹn / 615.992 khách (`GET /crm/report/appointment-statistics`, toàn bộ chi
// nhánh): cửa sổ 1 năm đọc 2.718.239 dòng InnoDB / ~5s, còn KHÔNG lọc ngày thì handler tự đặt
// `from=0, to=9999999999` ⇒ quét cạn bảng. Đây là hai ca người dùng chạm tới bằng ĐÚNG một cú
// bấm chuột (nút xoá ngày trên bộ lọc), không phải giả định.
//
// Vượt trần ⇒ trả 422 tường minh, KHÔNG tự cắt bớt dải ngày: cắt im lặng là trả thiếu số liệu
// mà người xem báo cáo không có cách nào biết.
const ReportMaxRangeSec int64 = 366 * 24 * 60 * 60
