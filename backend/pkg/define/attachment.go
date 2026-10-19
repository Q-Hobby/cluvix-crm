package define

// ref_type hợp lệ của bảng `attachment` (crm/attachment). Whitelist: giá trị lạ bị từ chối 422 —
// trước đây nhận chuỗi tuỳ ý (kể cả `medical_record`, bảng không tồn tại trong CRM ⇒ lỗi SQL).
const (
	AttachmentRefCustomer    = "customer"
	AttachmentRefAppointment = "appointment"
)

// AttachmentMaxFileMB — trần dung lượng 1 file đính kèm (MB). Body multipart cả request có trần
// riêng lớn hơn một chút (middleware.MaxMultipartRequestBytes) để file đúng trần vẫn lọt.
const AttachmentMaxFileMB = 32
