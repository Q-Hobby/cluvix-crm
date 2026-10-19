# Treatment (CRM)

> ⚠ **(đã bỏ, D15/CRM standalone)** File này mô tả dispatch form theo chuyên khoa/`department_type`,
> gắn với `user_emr_department`/EMR — verify 2026-09-28: module `emr` KHÔNG tồn tại trong
> `internal/modules/webapp/` của repo CRM standalone này, `user_emr_department`/`DEPARTMENT_TYPE`
> không còn trong code BE lẫn FE. Mọi mention `companyActiveId`/`GetCompanyActiveID` bên dưới cũng đã
> bỏ cùng company (D15). Giữ nguyên làm lịch sử/tham khảo, KHÔNG áp dụng.

## Mục lục

- order_service_used — 3 field đếm lần điều trị (HAY NHẦM)
- Tổng quan
- Tạo mới (Dental / Beauty)
- Cập nhật (Dental / Beauty)
- Xóa
- Check thanh toán trước khi tạo điều trị Beauty (FlowCount)
- % Hoàn thành điều trị — FlowCount
  - Công thức
  - Validate/derive `numTreatment` theo `serviceCategory_id` (chặn gốc — story-02)
- Commission Doctor (payment_commission.go)
- S3 File Management
- AppointmentNext (lịch hẹn kế tiếp gắn với điều trị)
- Labo gắn với điều trị (Dental — FlowPercent only)
  - Create flow
  - Update flow — `upsertLabo` (KHÔNG dùng DELETE+CREATE)
  - Response data

---


## order_service_used — 3 field đếm lần điều trị (HAY NHẦM)

| Field | Ý nghĩa |
|-------|---------|
| `quantity` | Tổng số lần **đã mua** |
| `numTreatment` | Số lần **được điều trị** theo gói |
| `numTreatmentOfService` | Số lần **đã thực hiện** thực tế |

Tiến trình hoàn thành: đọc thẳng `order_service_used.completion_percent` (cùng nguồn web/CRM, epic treatment-completion-standardization story-04); `-1` = sentinel không giới hạn buổi.
CRM FlowCount dùng `numTreatmentOfService < numTreatment` để check còn buổi hay không.

## Tổng quan
Liệu trình điều trị — ghi nhận quá trình điều trị khách hàng. Có 3 form FE: **FormDental** (nha khoa), **FormBeauty** (thẩm mỹ + generic count-based), **FormRehab** (phục hồi chức năng — có step-based KTV). Schema record + service create/update dùng chung endpoint Beauty cho FormBeauty và FormRehab. **List endpoint thống nhất** (xem § "List liệu trình theo customer").

## Tạo mới (Dental / Beauty)
- Auto code generation (TC prefix)
- `version = 1`
- Calculate: percent_complete, total amounts
- Set appointment link (appointment_id, appointmentNext_id)
- afterSave:
  - Tạo TreatmentCourseDetail[] từ FE data
  - Update OrderServiceUsed: link treatment_course_id, update treatment_status
  - Update Appointment: link treatment_course_id
  - Update Customer: status → STATUS_TREATMENT, last_visit_date
  - Tính commission cho doctor/technical/assistant
  - Log changes (LogTreatment)
  - **Auto-create CustomerCare** nếu `service.is_care = 1` (xem prompt care.md)

## Cập nhật (Dental / Beauty)
- **Optimistic locking**: version check → `version += 1`
- Recalculate percent_complete
- Update TreatmentCourseDetail[] (upsert)
- Recalculate OrderServiceUsed percent
- Update commission, log changes

## Xóa
- Check: không cho xóa nếu đã hoàn thành
- Revert OrderServiceUsed: unlink treatment_course_id, reset treatment_status
- Revert Appointment link
- Delete children: TreatmentCourseDetail, commission records
- Update Customer status, log delete

## Check thanh toán trước khi tạo điều trị Beauty (FlowCount)

Chỉ kích hoạt khi: **FlowCount + insert + category = TREATMENT + numTreatment > 0**.

```go
// ✅ ĐÚNG — dùng unit_price × quantity (giá lẻ trước discount)
totalListPrice := suData.UnitPrice.Mul(decimal.NewFromInt(int64(suData.Quantity)))
moneyPerTreatment := totalListPrice.Div(decimal.NewFromInt(int64(suData.NumTreatment))).Round(0)
moneyNeeded := decimal.NewFromInt(int64(newQty)).Mul(moneyPerTreatment)

// ❌ SAI — totalEnd đã trừ discount combo → moneyPerTreatment thấp hơn giá lẻ
moneyPerTreatment := suData.TotalEnd.Div(decimal.NewFromInt(int64(suData.NumTreatment))).Round(0)
```

**Lý do:** Mua 20 buổi giảm 20% → `totalEnd = 8M`, nhưng giá lẻ mỗi buổi = `unit_price × quantity / numTreatment`. Dùng `totalEnd` sẽ tính sai threshold (400k/buổi thay vì giá lẻ thực).

`ServiceUsedSnapshot` (repo) select cả `unit_price` từ `order_service_used`. Error key: `payment_required_before_treatment` (có đủ 4 locale vi/en/jp/fil).

## % Hoàn thành điều trị — FlowCount

Áp dụng cho mọi `order_service_used` KHÔNG thuộc FlowPercent (Beauty/Da liễu/YHCT/PHCN/Tâm thần/đa khoa mặc định — xem bảng flow ở đầu file). Trước epic treatment-completion-standardization (2026-08), `completion_percent` chỉ được tính đúng cho Dental (FlowPercent) — FlowCount bị bỏ trống/sai, khiến export/ZMA phải tự suy ra bằng công thức riêng (đã gỡ bỏ, xem story-03/04).

### Công thức

```go
// calcFlowCountCompletionPercent — crm/service/treatment_course.go:801
// completion_percent = round(numTreatmentOfService / numTreatment * 100)
// numTreatment == -1 (không giới hạn buổi) → completion_percent = -1 (sentinel, KHÔNG tính tỷ lệ)
// numTreatment nil hoặc <= 0 (dữ liệu bất thường) → trả nil, giữ nguyên completion_percent hiện tại
func calcFlowCountCompletionPercent(done int64, maxTreatment *int) *int
```

- **Sentinel `-1`**: hiển thị label "Không giới hạn" — BE dùng i18n key `service.unlimited` (`backend/pkg/i18n/vi.go`, đủ 4 locale vi/en/jp/fil) khi format cho export/response webapp. Kênh ZMA (`client/`) trả nguyên số `-1`, KHÔNG convert label ở BE (xem mục đầu file).
- **Nơi wiring**: `updateServiceUsedCommon` (`crm/service/treatment_course.go:818`, chạy sau MỌI calculator Dental/Beauty/Visit) — set `completion_percent` vào map `updates` CHỈ KHI chưa có (`if _, hasCompletionPercent := updates["completion_percent"]; !hasCompletionPercent` — dòng 863), để không ghi đè giá trị `calculatorDental` đã tự set riêng (MAX(percent_success_new)).
- **KHÔNG backfill lịch sử**: chỉ tính lại khi `order_service_used` có buổi điều trị mới/xoá buổi (tự nhiên qua `updateServiceUsedCommon` chạy lại) — record cũ chưa đổi giữ nguyên `completion_percent` cũ.
- ⚠️ **KHÔNG đụng `calculatorDental()`/`recalcPercentDental()`** (FlowPercent) khi sửa logic FlowCount — 2 nhánh tính hoàn toàn độc lập, chỉ dùng chung field đích `completion_percent` và guard "chưa có trong `updates`" để tránh xung đột ghi đè.

### Validate/derive `numTreatment` theo `serviceCategory_id` (chặn gốc — story-02)

`numTreatment` sai là nguyên nhân gốc khiến `completion_percent` sai — bị chặn ngay tại điểm tạo/sửa `order_service_used`, TRƯỚC KHI ghi DB:

| `serviceCategory_id` | Rule |
|---|---|
| `ServiceCategoryService` (1, Dịch vụ đơn lẻ) | `numTreatment` LUÔN ép về `1` — ghi đè payload FE nếu gửi khác, KHÔNG lỗi 422 |
| `ServiceCategoryTreatment` (3, Liệu trình) | Nếu FE KHÔNG gửi `numTreatment` → derive từ `service.numTreatment` (default trên bảng master `service`); nếu FE CÓ gửi → giữ nguyên giá trị FE |

Implement tại `validateOrderServiceUsed()` (`backend/internal/modules/webapp/crm/service/order_service_used_validate.go:32`, chữ ký nhận thêm `serviceCategoryId *int, serviceDefaultNumTreatment *int, numTreatmentSentByFE bool`), rule áp dụng ở dòng 71-82. Gọi từ **entry point duy nhất** `handleServices()` (`backend/internal/modules/webapp/crm/service/order_plan.go`, dùng chung cho cả `CreateOrder`/`UpdateOrder` — không có đường tắt bỏ qua validate):
- Dòng 947-961: load `service.serviceCategory_id, numTreatment` — chạy KHÔNG điều kiện (áp dụng cả create/update), tái sử dụng cho cả rule numTreatment lẫn logic `dayReExploitation` sản phẩm sẵn có.
- Dòng 1011: gọi `validateOrderServiceUsed(&svc, srv.ServiceCategoryId, srv.NumTreatment, numTreatmentSentByFE)`.

⚠️ **CRITICAL — dữ liệu cũ vi phạm rule "Dịch vụ đơn lẻ luôn numTreatment=1"**: khảo sát blast-radius trên DB dev (2026-08-03) cho thấy **41/484 record** (8.47%) có `serviceCategory_id=1` nhưng `numTreatment != 1` — đã được user xác nhận là dữ liệu lỗi cũ, KHÔNG backfill, chỉ áp rule cho record tạo/sửa MỚI. Khi debug case "sao completion_percent của dịch vụ đơn lẻ này không phải 1 buổi" → kiểm tra record có tạo TRƯỚC thời điểm rule này được bật hay không trước khi kết luận là bug.

## Commission Doctor (payment_commission.go)
- **Dental**: `createCommissionDoctorDental` — phân bổ payment theo từng treatment_course theo % hoàn thành
- **Beauty**: `createCommissionDoctorBeauty` — phân bổ payment theo từng treatment_course theo quantity
- Cả 2 đều query `treatment_course_doctor` để lấy `totalPayment` đã tính
- **GORM Pitfall**: column trong `treatment_course_doctor` là `payment` (KHÔNG phải `amount`)
  - Raw SQL phải dùng `tcd.payment`, không dùng `tcd.amount`
- **GORM Pitfall**: `treatment_course_doctor` KHÔNG có cột `percent` — "% điều trị" phải tính
  `treatment_course.percent_success_new - treatment_course.percent_success` (xem FE
  `gift-doctor.html` dùng đúng công thức này, KHÔNG đọc field `percent` trên model)

## S3 File Management
- `list-folder` — List folders (S3)
- `create-folder` — Create folder (S3)
- `list-file-treatment` — List files (S3)
- `create-presigned-url` — Presigned URL (S3)
- `upload-file-treatment` — Upload file (S3)

## AppointmentNext (lịch hẹn kế tiếp gắn với điều trị)
- FE form-dental/form-beauty gom `appointmentDate / appointmentType / appointmentDoctor / noteNext` vào FormGroup `appointmentNext`
- BE input: `AppointmentNext *AppointmentNextInput` (struct riêng), service `saveAppointmentNext` tạo record `appointment` mới + set `treatment_course.appointmentNext_id`
- List endpoint (`get-treatment-course-of-customer`, ListTreatmentCourse) phải attach `appointmentNext` với SELECT đủ field FE form đọc: `id, code, appointment_date, status, appointment_type_id, doctorAppointment_id, content`
- Field `content` của appointment map sang `noteNext` trên FE (form control `noteNext`)
- Edit mode: nếu `appointmentNext.status != NOT_CHECKIN` → FE disable toàn bộ control trong group (không cho sửa lịch đã check-in)

## Labo gắn với điều trị (Dental — FlowPercent only)
- FE form-dental gom labo vào FormGroup `supplierLaboOrder = { id, id_supplier_labo, id_doctor, date_receiver, labo_status, unit_id, noteLabo }`
- `unit_id` **bắt buộc** từ FE — pre-fill từ `service.unit_id` (cả create + edit, vì edit mode dịch vụ disable nên `onChangeServiceUsed` không chạy)
- BE input: `SupplierLaboOrder *SupplierLaboInput` (nested), validate đủ data qua `hasLaboInput()` (cần `id_supplier_labo + id_doctor + date_receiver`)
- DB schema: `id_receiver_labo` đã đổi sang **NULL** (migration `20260512100000`) — module labo riêng tự gán khi giao nhận, treatment form không quản
- Date parse: hỗ trợ `2006-01-02` + `RFC3339` + `RFC3339Nano` (FE Date → JSON ISO có fractional seconds)

### Create flow
- Hàm `createLabo`: INSERT mới, `IdReceiverLabo = nil`

### Update flow — `upsertLabo` (KHÔNG dùng DELETE+CREATE)
| Input từ FE | Action |
|---|---|
| `id != nil` + đủ data | UPDATE chỉ 6 field FE quản (`id_supplier_labo, id_doctor, date_receiver, labo_status, unit, note`) + `updated_at` |
| `id == nil` + đủ data | INSERT mới (qua `createLabo`) |
| `id != nil` + thiếu data | DELETE record (user xóa labo trên form) |
| `id == nil` + thiếu data | no-op |

**Tại sao không DELETE+CREATE:** giữ `id_receiver_labo` (module labo set), audit `created_at/by`, `version`, FK references (payment, log labo). DELETE+CREATE từng wipe `id_receiver_labo` — bug đã fix.

### Response data
- `loadTreatmentWithRelations` (response create/update) + list endpoints attach `supplierLaboOrder` với SELECT: `id, treatment_course_id, id_supplier_labo, id_doctor, date_receiver, labo_status, unit as unit_id, note`
- DB column `unit` alias `unit_id` để khớp FE form control

---
