package define

// Enum của dịch vụ dưới dạng (giá trị, nhãn tiếng Việt) — MỘT nguồn sự thật dùng chung cho:
//   - sheet "Danh_muc" trong file export và file mẫu import,
//   - dropdown trong file mẫu import,
//   - resolver khi import đọc ngược nhãn người dùng chọn/gõ.
//
// Nhãn phải khớp đúng thứ user thấy trên web (FE `define/service.define.ts`,
// `ServiceAdultChildPipe`): file mẫu ghi "Trẻ em" mà web hiểu là "Người lớn" thì user điền đúng
// theo file vẫn ra dữ liệu sai, mà không có lỗi nào bật lên. Đúng loại lệch từng tồn tại trong
// ghi chú template cũ (child bị ghi ngược, time_expired_type ghi 2=tháng trong khi app hiểu 2=năm).

// ServiceEnumOption — 1 lựa chọn enum.
type ServiceEnumOption struct {
	Value int
	Label string
}

// ServiceCategoryOptions — Danh mục (dịch vụ / sản phẩm / liệu trình / phí khám).
var ServiceCategoryOptions = []ServiceEnumOption{
	{ServiceCategoryService, "Dịch vụ"},
	{ServiceCategoryProduct, "Sản phẩm"},
	{ServiceCategoryTreatment, "Liệu trình"},
	{ServiceCategoryFee, "Phí tư vấn"},
}

// ServiceChildOptions — Nhóm tuổi. Khớp FE SERVICE_FOR_OPTIONS: 1 = Người lớn, 2 = Trẻ em.
// Để TRỐNG nghĩa là không phân biệt — cố ý không có lựa chọn "Tất cả" vì web cũng không có.
var ServiceChildOptions = []ServiceEnumOption{
	{1, "Người lớn"},
	{2, "Trẻ em"},
}

// ServiceCommissionTypeOptions — cách tính hoa hồng. Khớp FE COMMISSION_TYPE_OPTIONS.
var ServiceCommissionTypeOptions = []ServiceEnumOption{
	{1, "Số tiền"},
	{2, "%"},
}

// ServiceFlagOptions — cột cờ bật/tắt (Chăm sóc, Cho sửa NVL, Trả góp).
var ServiceFlagOptions = []ServiceEnumOption{
	{1, "Có"},
	{0, "Không"},
}

// ServiceStatusOptions — trạng thái bản ghi.
var ServiceStatusOptions = []ServiceEnumOption{
	{1, "Đang dùng"},
	{0, "Ngưng"},
}
