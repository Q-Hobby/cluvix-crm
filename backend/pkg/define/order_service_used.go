package define

// OrderServiceUsed isLock
const (
	ServiceUsedNotLock = 1
	ServiceUsedIsLock  = 2
)

// OrderServiceUsed service_discount_type
const (
	DiscountTypeNormal  = 1 // VND
	DiscountTypePercent = 2 // %
)

// OrderServiceUsed payment_status
const (
	ServicePaymentStatusStart  = 1
	ServicePaymentStatusFree   = 2
	ServicePaymentStatusPartly = 3
	ServicePaymentStatusDone   = 4
	ServicePaymentStatusRefund = 5
)

var ServicePaymentStatusLabels = map[int64]string{
	ServicePaymentStatusStart:  "Chưa thanh toán",
	ServicePaymentStatusFree:   "Miễn phí",
	ServicePaymentStatusPartly: "Thanh toán 1 phần",
	ServicePaymentStatusDone:   "Hoàn thành thanh toán",
	ServicePaymentStatusRefund: "Hoàn trả dịch vụ",
}

// category_id mặc định của dòng dịch vụ
const (
	CategoryTreatment = 1
)

// Service serviceCategory_id
const (
	ServiceCategoryService   = 1
	ServiceCategoryProduct   = 2
	ServiceCategoryTreatment = 3
	ServiceCategoryFee       = 4 // Phí tư vấn — không tính commission/KPI
)

// QuantityGateFixCutoverAt: mốc thời gian (Unix, giây) fix gate/Revenue cho
// ServiceCategoryService dùng Quantity làm mẫu số/giới hạn buổi điều trị thay
// NumTreatment (NumTreatment bị ép cứng =1 cho category này, xem
// order_service_used_validate.go). order_service_used có created_at TRƯỚC mốc
// này giữ nguyên hành vi cũ — KHÔNG mở lại order đang bị chặn, KHÔNG backfill
// Revenue/hoa hồng đã tính (quyết định user, xem
// skill/bmad/order-service-quantity-numtreatment-fix/brief.md mục Q1/Q2).
// Giá trị hiện là placeholder (00:00:00 UTC 2026-08-08, ngày story-01 được code)
// — PM/Ops PHẢI cập nhật đúng ngày deploy fix thật trước khi release, KHÔNG
// dùng time.Now() (giá trị phải cố định qua các lần build/deploy).
const QuantityGateFixCutoverAt int64 = 1786147200
