package define

// SuperRoles — role có TẤT CẢ permission, không cần manual assign từng cái.
// Phải khớp với frontend/src/app/define/rbac.define.ts SUPER_ROLES.
var SuperRoles = []string{"admin", "develop"}

// RoleAdmin — role gốc, không cho phép xoá kể cả bởi user có permission deleteRole
// (khác super-user check: user thường được phân công deleteRole vẫn xoá được role khác).
const RoleAdmin = "admin"

// PermManageConfig — quyền DUY NHẤT của module Cấu hình (epic config-single-permission 2026-09-02:
// 69 key cũ — 7 họ Config*, Supplier, Warehouse, CashFlowCategory, configEInvoice, manageVitalType,
// manageMedRoute, RecordTemplate, ClsTemplate — gộp về key này; 669 entry route_perm_map.json dùng nó).
// Thay cho PermUpdateConfigClinic/PermCreateConfigClinic/PermExtConfigClinic đã xoá.
//
// Vẫn cần guard TƯỜNG MINH trong handler (không chỉ dựa route_perm_map): PermissionEnforcer là
// middleware toàn cục nhưng đang audit-mode (PERM_ENFORCE=false) nên chỉ log would-deny, KHÔNG chặn.
// Các điểm ghi nhạy cảm ở module config (mapping byt_code/mã chuyên khoa, export, import Excel ghi
// giá/hoa hồng) phải tự kiểm quyền tại handler.
const PermManageConfig = "manageConfig"

// PermCreateUser / PermUpdateUser / PermExtUser — quyền nhân sự (FE permission taxonomy, khớp
// permissions.gen.json). Dùng cho guard tường minh trong handler `hr/user`: middleware permission
// đang chạy audit-mode (PERM_ENFORCE=false) nên KHÔNG chặn, mà import/export nhân sự là hai đường
// tạo tài khoản + đọc trọn danh sách nhân sự — không thể chỉ dựa vào *hasPerm phía FE.
const (
	PermCreateUser = "createUser"
	PermUpdateUser = "updateUser"
	PermExtUser    = "extUser"
)

// PermExtPayment — quyền xuất Excel danh sách phiếu thanh toán (FE `permissionExtPayment`, khớp
// permissions.gen.json). Guard tường minh trong `crm/payment` handler khi `_export=1` (middleware
// permission đang audit-mode, route chỉ gác viewPayment).
const PermExtPayment = "extPayment"

// PermExtCustomer / PermExtAppointment / PermManageCrmReport — quyền xuất Excel (S-7, 2026-10-18),
// khớp permissions.gen.json + FE (`permissionExtCustomer`, `permissionExtAppointment`,
// `permissionManageCrmReport`). Gác `?_export=1` qua middleware.RequireExportPerm (chặn thật, không
// audit-mode). Catalog KHÔNG có ext riêng cho báo cáo ⇒ export báo cáo dùng manageCrmReport (đúng quyền
// FE đang gác nút Export ở các màn báo cáo); KHÔNG có extPlanOrder ⇒ export Quản lý đơn hàng dùng
// PermExtPayment (màn port từ nhóm kế toán cùng 6 màn phiếu thu, chở doanh thu/công nợ + SĐT khách).
const (
	PermExtCustomer     = "extCustomer"
	PermExtAppointment  = "extAppointment"
	PermManageCrmReport = "manageCrmReport"
)

// IsSuperRole trả true nếu name nằm trong whitelist super role.
func IsSuperRole(name string) bool {
	for _, r := range SuperRoles {
		if r == name {
			return true
		}
	}
	return false
}
