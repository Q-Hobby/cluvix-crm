# Gotchas HR Department constants

## Mục lục

- `pkg/define/hr.go` là NGUỒN SỰ THẬT DUY NHẤT — đừng khai hằng local
- ⚠ Lịch sử: `Convert`/`Marketing` từng khai SAI, đã sửa 2026-07-30
- Tài liệu cũ đã bị thay thế

---


## `pkg/define/hr.go` là NGUỒN SỰ THẬT DUY NHẤT — đừng khai hằng local

```go
HrDepartmentHr               = 1
HrDepartmentReceptionist     = 2
HrDepartmentTelesale         = 3
HrDepartmentMarketing        = 4
HrDepartmentAccountant       = 5
HrDepartmentCustomerCare     = 6
HrDepartmentDoctor           = 7
HrDepartmentAssistant        = 8
HrDepartmentBoardOfDirectors = 9
HrDepartmentDirectSale       = 10
HrDepartmentConvert          = 11
HrDepartmentSaleStaff        = 12
HrDepartmentTechnical        = 13
```

Khớp bảng `hr_department` thật và `frontend/src/app/define/user.define.ts` `DEPARTMENT_SELECT`. Đủ 13 phòng ban, **không còn phòng nào phải hardcode** — cần id phòng ban thì import từ `pkg/define`, KHÔNG khai lại hằng local trong module (đẻ nguồn sự thật thứ hai).

## ⚠ Lịch sử: `Convert`/`Marketing` từng khai SAI, đã sửa 2026-07-30

Trước commit `7e14ef1d` (2026-07-30): `HrDepartmentConvert = 1`, `HrDepartmentMarketing = 2` — sai so với DB thật (`1` = HR, `2` = Lễ tân). Đã sửa về `11`/`4`.

**Việc sửa này ĐỔI HÀNH VI, không phải dọn dẹp thuần** — vì module `sale/leadscope` (đã bỏ khỏi CRM standalone) từng dùng `HrDepartmentConvert` để quyết định lọc lead:

| | Trước 30/07 | Sau 30/07 |
|---|---|---|
| Nhân sự phòng **HR (1)** | bị tính nhầm là Convert → lead bị lọc theo `convert_id` | hết bị lọc |
| Nhân sự phòng **Convert thật (11)** | KHÔNG bị lọc, thấy hết lead | bắt đầu bị lọc theo `convert_id` |

Docblock trong `pkg/define/hr.go` ghi lại nguyên văn quyết định này (user chốt), kèm trỏ `deferred-work.md` DW-38.

⚠ Khi điều tra bug "user X trước xem được giờ không xem được lead/khách hàng" → **mốc 2026-07-30 là nghi phạm đầu tiên**, đặc biệt với nhân sự phòng Convert. Cộng hưởng với story-32 cùng ngày (`GetCustomer` bắt đầu áp `leadscope`) đã gây ra ca 404 ở `CustomerDetail` — xem `skill/skills/managing-crm-options/reference/option-crm-options.md` §Option 5.

## Tài liệu cũ đã bị thay thế

Bản trước của file này ghi "Convert = 1 là SAI, **KHÔNG sửa** vì leadscope đang dùng" và khuyên hardcode `11`/`4` thay vì import hằng số. **Lời khuyên đó nay SAI ngược** — hằng số đã đúng, hardcode lại mới là lỗi. Ví dụ nó dẫn (`customerFormLockDeptConvert` trong `crm/customer/service/service.go`) đã bị xoá đúng theo hướng này; comment còn lại ở `service.go:17-19` giải thích lý do xoá.
