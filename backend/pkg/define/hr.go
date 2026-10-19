package define

// HrDepartment types — id theo bảng `hr_department` (ground truth DB), khớp
// `frontend/src/app/define/user.define.ts` DEPARTMENT_SELECT. Đây là NGUỒN SỰ THẬT
// DUY NHẤT cho id phòng ban ở BE — đừng khai lại hằng local trong module.
//
// ⚠ 2026-07-30 (user chốt): `Convert` và `Marketing` trước đây khai SAI (1 và 2 —
// thực chất id 1 = hr, id 2 = receptionist). `Convert` khi đó được scope lọc lead
// (module sale cũ, đã xoá ở crm-standalone story-08) dùng ⇒ sửa lần này ĐỔI phạm vi lead nhìn thấy được: trước đây người phòng HR bị
// tính là convert, người convert thật thì không. Xem `deferred-work.md` DW-38 +
// query kiểm trước deploy.
const (
	HrDepartmentHr               = 1
	HrDepartmentReceptionist     = 2
	HrDepartmentTelesale         = 3
	HrDepartmentMarketing        = 4
	HrDepartmentAccountant       = 5
	HrDepartmentCustomerCare     = 6
	HrDepartmentBoardOfDirectors = 9
	HrDepartmentConvert          = 11
	HrDepartmentSaleStaff        = 12
)
