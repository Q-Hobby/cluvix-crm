export const displayColsReceptionList = [
  { label: 'ID', value: 'id' },
  // Nhóm "Khách hàng" tách 3 cột riêng để filter/sort độc lập
  { label: 'Khách hàng', labelKey: 'common.label.patient', value: 'patient' },
  { label: 'Giới tính · Tuổi', labelKey: 'displaycols.reception.gender_age', value: 'gender' },
  { label: 'Số điện thoại', labelKey: 'common.label.phone', value: 'phone' },
  // Nhóm "Thông tin cơ sở" tách 3 cột riêng
  { label: 'Ngày đến', labelKey: 'common.label.visit_date', value: 'appointment_date' },
  { label: 'Nhân viên phụ trách', labelKey: 'common.label.doctor', value: 'doctor' },
  { label: 'Trạng thái', labelKey: 'common.label.status', value: 'status' },
  { label: 'Chỉ số cơ thể', labelKey: 'common.label.body_metrics', value: 'body_metrics' },
  { label: 'Chỉ số sinh tồn', labelKey: 'common.label.vital_signs', value: 'vital_signs' },
  { label: 'Nội dung hẹn', labelKey: 'common.label.appointment_content', value: 'appointment_content' },
  // Nhóm "Thông tin lâm sàng" tách 3 cột riêng để filter/sort + ẩn/hiện độc lập
  { label: 'Lý do', labelKey: 'common.label.reason', value: 'chief_complaint' },
  { label: 'Triệu chứng', labelKey: 'common.label.symptoms', value: 'symptoms' },
  { label: 'Tiền sử', labelKey: 'emr.reception_list.label_history', value: 'medical_history' },
  { label: 'Địa chỉ', labelKey: 'common.label.address', value: 'address' },
  { label: 'Chi nhánh', labelKey: 'common.label.branch', value: 'branch' },
  { label: 'Thương hiệu', labelKey: 'common.label.company', value: 'company' },
  { label: 'Người tạo', labelKey: 'common.label.user_created', value: 'created_by' },
  { label: 'Ngày tạo', labelKey: 'common.label.created_at', value: 'created_at' },
];

/**
 * Default columns hiển thị — gọn 7 cột vừa khít 1366px, không scroll ngang.
 * Tiếp tân/BS lookup nhanh + thao tác check-in/cancel/detail trên 1 view.
 * Các cột chi tiết lâm sàng (body/vitals/medical_info...) hide mặc định,
 * BS bật qua column toggle khi cần.
 */
export const selectedColumnsReceptionList: any = [
  'patient',
  'gender',
  'phone',
  'appointment_date',
  'doctor',
  'status',
];

