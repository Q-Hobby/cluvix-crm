# Import Module

## Mục lục

- Hiện trạng code
- Cấu trúc file Excel + normalize
- Logic INSERT vs UPDATE
  - Defaults khi INSERT (không đọc từ file)
- Note tự động khi INSERT
- Transaction

---


Module: `internal/modules/webapp/import/`
Endpoint: `POST /api/import/customer/from-excel`
Handler: `internal/modules/webapp/import/customer/handler/import.go`


---

## Hiện trạng code

- **KHÔNG nhận file upload** (cố ý — công cụ ops chạy tay) — handler đọc file trên server theo env `IMPORT_CUSTOMER_FILE` (`define.EnvImportCustomerFile`, đường dẫn tuyệt đối); env rỗng ⇒ 422 `import_file_not_configured`. KHÔNG nhận đường dẫn từ request (đọc file tuỳ ý trên server). Trước 2026-10-17 đường dẫn gán cứng trong code.
- `defaultCountryID = 237` (hardcode trong `handler/helpers.go`; `defaultCompanyID` đã bỏ cùng company, D15)

---

## Cấu trúc file Excel + normalize

Cấu trúc: Row 0 label / Row 1 tên cột DB / Row 2+ data; normalize phone 9 số, TitleCase, birthday DD/MM/YYYY, created_day.

**Allowlist & defaults:**
- Allowlist hiện tại 11 cột: `created_day, old_code, fullname, phone, gender, customerType, birthday, address, source_id, payment_user_id, branch_id` — cột ngoài allowlist bỏ qua (chống column injection)
- `address1` / `address2` ở Row 1 → không vào DB, ghép vào `address` bằng `", "`
- `phone` > 20 ký tự, hoặc chứa ký tự không phải số và > 15 ký tự → drop field
- `gender` và các FK int (`intCols`) parse qua `ParseFloat` (nhận cả `"2.0"`); không parse được → lỗi dòng + rollback. KHÔNG nhận chuỗi `'Nam'/'Nữ'`
  - Đây là bẫy chung của MỌI importer, không riêng module này: file soạn trên Google Sheets/LibreOffice ghi số nguyên thành `"3.0"`. Luật đầy đủ + ranh giới an toàn với cột tiền: `skill/skills/debugging-backend-queries/reference/patterns-bulk-import.md` §8.

---

## Logic INSERT vs UPDATE

```
Lookup customer theo (phone AND fullname) trong tenant scope (ScopedDBNullBranch)
→ tìm thấy: UPDATE record đó (version += 1)
→ không: INSERT mới
```

Không có nhánh UPDATE theo `id` — allowlist không có cột `id`.

### Defaults khi INSERT (không đọc từ file)

| Field | Giá trị |
|-------|---------|
| `country_id` | `237` |
| `status` | `define.CustomerStatusOldSystem` |
| `customerType` | `2` (khách cũ) nếu file không có |
| `data_id` | `2` (dữ liệu cũ) |
| `code` | không gen — tránh lệch counter `config_code_monthly` |

---

## Note tự động khi INSERT

Cột có label ở Row 0 nhưng Row 1 trống → gom thành 1 note HTML (`<p><strong>label:</strong> value</p>`) insert thẳng bảng `note` bằng raw map (lấy id qua `LAST_INSERT_ID()`, không qua struct hooks — tránh side effect `AfterCreate`).
Label tiền (`Tổng tiền đã tiêu dùng`, `Công nợ`) → `formatVND` (strip dấu phẩy nghìn, thêm `.` + `₫`).

---

## Transaction

→ toàn file 1 transaction, dòng lỗi → rollback toàn bộ (`import_row_error`). Re-run an toàn nhờ lookup phone+fullname.
