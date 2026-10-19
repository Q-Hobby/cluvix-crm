# Routes Ops

## Mục lục

  - Print (internal/modules/webapp/print/router.go) — ~16 endpoints (authed)
  - History (internal/modules/webapp/history/router.go) — 2 endpoints (authed)
  - Import (internal/modules/webapp/import/router.go) — 1 endpoint (authed)
  - Export (internal/modules/webapp/export) — KHÔNG có HTTP route riêng

---


> ⚠ Path viết trước restructure 2026-07-28: `internal/modules/<domain>` đọc là `internal/modules/webapp/<domain>` (CRM standalone chỉ còn nhóm `webapp/`).

> Index tổng + auth groups: `arch-routing.md`. Nguồn: code tại 2026-06-10.

---

### Print (internal/modules/webapp/print/router.go) — ~16 endpoints (authed)

**Path prefix:** `/api/print`. Output thường là HTML để in/PDF.

- GET `/api/print/order-plan/:id` → `PrintOrderPlan` (internal/modules/webapp/print/order_plan/handler/handler.go:23) — in phiếu đơn hàng điều trị.
- GET `/api/print/deposit/:id` → `PrintDeposit` (internal/modules/webapp/print/payment/handler/handler.go:71).
- GET `/api/print/refund-consignment/:id` → `PrintRefundConsignment` (internal/modules/webapp/print/payment/handler/handler.go:31) — in phiếu hoàn ký gửi; alias `refund-deposit/:id` (cùng data shape).
- GET `/api/print/refund-service-treatment/:id` → `PrintRefundServiceTreatment` (internal/modules/webapp/print/payment/handler/handler.go:270).
- GET `/api/print/payment/:id` → `PrintPayment` (internal/modules/webapp/print/payment/handler/handler.go:96) — in phiếu thu.
- GET `/api/print/multi-payment` → `PrintMultiPayment` (internal/modules/webapp/print/payment/handler/handler.go:171) — in gộp nhiều payment.
- GET `/api/print/payment-in-day` → `PaymentInDay` (internal/modules/webapp/print/payment_in_day/handler/handler.go:27).
- GET `/api/print/reception-payment` → `PrintReceptionPayment` (internal/modules/webapp/print/reception_payment/handler/handler.go:26) — in phiếu thu lễ tân (theo filter).
- GET `/api/print/appointment-byhour-list` → `PrintAppointmentByhourList` (internal/modules/webapp/print/appointment/handler/handler.go:39) — DS lịch hẹn theo giờ; kèm `-export` → Excel (internal/modules/webapp/export/appointment.go).
- GET `/api/print/appointment-bydoctor-list` → `PrintAppointmentBydoctorList` (internal/modules/webapp/print/appointment/handler/handler.go:74) — DS lịch hẹn theo bác sĩ × giờ; kèm `-export` → Excel.

---

### History (internal/modules/webapp/history/router.go) — 2 endpoints (authed)

**Path prefix:** `/api/history`.

- GET `/api/history/history-export` → `ListHistoryExport` (internal/modules/webapp/history/history_export/handler/handler.go:21) — lịch sử file export.
- GET `/api/history/log` → `ListLog` (internal/modules/webapp/history/log/handler/handler.go:24) — audit log gộp customer/appointment/payment/treatment.

---

### Import (internal/modules/webapp/import/router.go) — 1 endpoint (authed)

**Path prefix:** `/api/import`.

- POST `/api/import/customer/from-excel` → `ImportCustomerFromExcel` (internal/modules/webapp/import/customer/handler/import.go:39).

---

### Export (internal/modules/webapp/export) — KHÔNG có HTTP route riêng

Utility module — hàm export Excel/PDF gọi cross-module (Print, Med report, Workspace...). Cover các domain: appointment, broker, config, coupon, customer, med_*, order, payment, payment_branch, report_revenue, service, treatment, unit, user, workspace.

---
