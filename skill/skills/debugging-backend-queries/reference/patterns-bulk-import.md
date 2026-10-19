# Bulk Import Excel

## Mục lục

- 1. Format Excel — 2 header row
- 2. FK cột nhận số HOẶC tên/mã — batch resolve, không N+1
- 3. Số nguyên dương làm FK PHẢI batch-verify tồn tại (CRITICAL)
- 4. Accumulate lỗi theo dòng, KHÔNG fail-fast — báo 1 lần qua dialog
- 5. Lỗi lạ/không lường trước — SanitizeErrorMessage, KHÔNG echo raw error
- 6. Đọc ngày/serial từ Excel (CRITICAL)
- 7. Đọc tiền/số từ Excel (CRITICAL)
- 8. Đọc SỐ NGUYÊN/FK từ Excel — bẫy "3.0" (CRITICAL)
- 9. Cột ngày bị chủ động loại khỏi import — audit trước khi bật lại
- FE tương ứng

---


Dùng khi import Excel có cột FK (company/supplier/warehouse...) và cần báo lỗi tường minh nhiều dòng
1 lần, thay vì fail-fast từng dòng..

## 1. Format Excel — 2 header row

- Row 1 = nhãn hiển thị (người đọc) · Row 2 = field key (machine, khớp tên cột parser dùng) · Row 3+ = data
- Parser build `colIdx map[string]int` từ row 2, đọc data qua closure `col(row, "field_key")` — KHÔNG hardcode index cột cố định
- Check `requiredKeys` tồn tại trong row 2 trước khi parse data

## 2. FK cột nhận số HOẶC tên/mã — batch resolve, không N+1

```go
// Bước 1: gom mọi giá trị KHÔNG phải số của 1 cột FK trên toàn bộ file
companyNames := collectNonNumeric("company_id") // map[string]struct{}

// Bước 2: 1 query/bảng cho toàn bộ tên gom được (KHÔNG query per-row/per-distinct-value)
batchResolveFK("company", []string{"name", "code"}, companyNames)
// → h.DB.Table(table).Where("name IN ? OR code IN ?", list, list).Scan(&results)
```

- `resolveFK(table, raw)`: `raw` là số → verify tồn tại (mục 3); không phải số → lookup map đã batch-resolve
- Áp dụng cho mọi cột FK-by-name trong 1 file (company/supplier/warehouse...), không riêng lẻ từng cột

## 3. Số nguyên dương làm FK PHẢI batch-verify tồn tại (CRITICAL)

Chấp nhận số bất kỳ làm ID mà không verify tồn tại → lỗi chỉ lộ ra lúc `tx.Create` dưới dạng MySQL FK
1452 thô (tên bảng/constraint nội bộ), không dịch được, không nhóm được vào đúng loại lỗi.

```go
verifiedNumericIds := map[string]map[int]bool{
    "company":  batchVerifyIds("company", collectNumericIds("company_id")),
    "supplier": batchVerifyIds("supplier", collectNumericIds("supplier_id")),
}
// batchVerifyIds: h.DB.Table(table).Where("id IN ?", ids).Pluck("id", &found) — 1 query/bảng

resolveFK := func(table, raw string) (int, bool) {
    if id, err := strconv.Atoi(raw); err == nil && id > 0 {
        if verified, needsCheck := verifiedNumericIds[table]; needsCheck {
            return id, verified[id] // bảng có bật check → phải thật sự tồn tại
        }
        return id, true // bảng không có trong map → downstream tự verify riêng (vd warehouse qua WarehouseBelongsToCompany)
    }
    // ... lookup theo tên
}
```

## 4. Accumulate lỗi theo dòng, KHÔNG fail-fast — báo 1 lần qua dialog

```go
addIssue := func(errType, label, msg string) { issues = append(issues, issue{errType, label, msg}) }

// Mỗi lỗi trong loop: addIssue(...) rồi `continue` sang dòng tiếp — KHÔNG return sớm.
// Dòng lỗi bị loại khỏi kết quả parse (không tạo record) nhưng không chặn dòng khác.

if len(issues) > 0 {
    // Group theo errType, respond 1 lần — KHÔNG tạo record nào (kể cả dòng hợp lệ) khi có bất kỳ lỗi
    response.ValidateWithDetails(c, i18n.TF(c, "import_has_errors", len(issues)), details)
    return
}
```

- `errType` dùng **const string** (`errTypeCompany`, `errTypeMaterial`...), không hardcode literal rải rác — typo → label rỗng, không lỗi biên dịch nếu dùng string tay
- `response.ImportErrorGroup{Type, Label, Items}` + `response.ValidateWithDetails(c, message, details)` — 422 `{message, details}`, dùng CHUNG cho: bulk-FK-missing, per-row parse error, và lỗi lúc `CreateInTx` (tx-error cũng đi qua format này, kể cả chỉ có 1 item) để FE xử lý đồng nhất
- Structural pre-check (file rỗng, thiếu cột, sai định dạng) trước khi vào loop cũng phải đi qua `ValidateWithDetails(c, msg, nil)` — KHÔNG dùng `response.Validate` cũ — để toàn bộ response lỗi của endpoint dùng chung 1 shape

## 5. Lỗi lạ/không lường trước — SanitizeErrorMessage, KHÔNG echo raw error

```go
if _, err := h.StockEntrySvc.CreateInTx(tx, &input, userID); err != nil {
    msg := response.SanitizeErrorMessage(c, err) // dịch sentinel/LocalizedError; MySQL 1451/1452/1062... → message an toàn
    return errs.NewLocalizedError("import_row_error", first.rowIdx, msg)
}
```

`response.SanitizeErrorMessage(c, err) string` (`pkg/response`) — như `HandleError` nhưng trả string thay vì
ghi response ngay, dùng khi cần gộp message vào 1 payload lớn hơn (dialog nhiều lỗi). Lỗi không dịch được
(không phải sentinel/LocalizedError, không phải MySQL code đã biết) → fallback `i18n.T(c, "server_error")`,
KHÔNG bao giờ trả `err.Error()` thô (rò rỉ tên bảng/constraint nội bộ).

## 6. Đọc ngày/serial từ Excel (CRITICAL)

- Ô ngày có **ngày ≤ 12** (`6/3/1990`) bị Excel tự nhận là date → lưu serial number; `f.GetRows()` mặc định render lại theo locale (`06-03-90`) → parse fail. Ngày > 12 (`19/12/1995`) Excel giữ text nên vào được → **miss chọn lọc**, rất khó phát hiện.
- **LUÔN** đọc bằng `f.GetRows(sheet, excelize.Options{RawCellValue: true})` → ô date trả về serial number, độc lập locale. Serial = số ngày kể từ epoch `1899-12-30`: `time.Date(1899,12,30,0,0,0,0,time.UTC).AddDate(0,0,serial)`.
- `time.Parse("02/01/2006", ...)` **bắt buộc zero-pad 2 chữ số** → fail với ngày/tháng 1 chữ số (`19/3/1990`). Parse tay: split separator `/ - .`, không bắt zero-pad; phần đầu 4 chữ số → `YYYY-MM-DD`, ngược lại `DD-MM-YYYY` (chuẩn VN); validate ngày thực (loại `31/02`).
- Số nguyên nhỏ (1900–2100) ở cột ngày = **năm gõ nhầm cột** → drop, KHÔNG đọc thành serial (ra rác ~1905). Ngưỡng serial hợp lệ `3000 ≤ n < 500000`; `n ≥ 500000` coi là Unix timestamp.
- **Dùng chung `pkg/exceldate`** cho MỌI handler import có cột date — KHÔNG tự viết parser riêng:
  - `ParseDate(s, loc) (time.Time, bool)` — ngày không giờ.
  - `ParseDateString(s) (string, bool)` — trả `"2006-01-02"` (UTC), cho cột lưu string như `birthday`.
  - `ParseDateTime(s, loc) (time.Time, bool)` — có giờ + Unix timestamp, cho cột lưu unix như `created_day`.
  - `SerialToTime`, hằng `SerialMin`/`SerialMax`.
  - `loc` lấy từ `tz.LocFromContext(c, db)` (hoặc `tz.Location(db, companyID)`) — KHÔNG `time.Local`.
- Đã migrate: `customerimport` (lead), `import/customer`, `inventory/stock_entry`, `hr_attendance` (fix thứ tự M/D→DD/MM sai template), `marketing/digital_facebook` (bỏ `time.Local`).

## 7. Đọc tiền/số từ Excel (CRITICAL)

- Parse tiền: **KHÔNG** `decimal.NewFromString(s)` thẳng — `"1.000"` (VN/MISA dùng `.` ngăn nghìn) bị đọc thành `1.0`, lệch **1000×** số tiền kế toán, build không bắt.
- Chỉ parse thẳng khi chuỗi **không có** `.` và `,`. Có dấu phân cách → heuristic: cả `.` lẫn `,` → dấu XUẤT HIỆN SAU là thập phân; chỉ một loại → ≤2 chữ số sau = thập phân, còn lại = ngăn nghìn (bỏ đi). Xem `misa_conversion/handler.parseMoney` (có unit test).

## 8. Đọc SỐ NGUYÊN/FK từ Excel — bẫy "3.0" (CRITICAL)

Hệ quả trực tiếp của §6: `RawCellValue: true` trả về **đúng chuỗi nằm trong XML**. Excel desktop ghi
số nguyên là `<v>3</v>`, nhưng **Google Sheets / LibreOffice ghi `<v>3.0</v>`**. File đi qua bảng
tính khác là mọi parser số trượt hết, mà không cột nào báo "sai kiểu":

| Parser | Nhận `"3.0"` | Hậu quả |
|---|---|---|
| `helpers.ToInt64` (chỉ `ParseInt`) | `0` | id/FK biến mất |
| `parseCatalogRef` | `(id:0, name:"3.0")` | đi tra danh mục theo **TÊN** `"3.0"` |
| `parseEnumValue` | không hợp lệ | đòi user gõ nhãn (`"Đang dùng"`) dù `1` là thứ export vừa ghi ra |
| `parseImportUint` | từ chối | lỗi dòng "phải là số" |

Chuẩn hoá `"N.0" → "N"` **trước** khi vào parser. Hai cách đã dùng trong repo:
- `import/customer/handler/import.go` §`intCols` — `strconv.ParseFloat` rồi `int(fv)`.
- `config/service/handler/helpers.go` → `normalizeExcelIntCell(col, raw)` — regex, có deny-list cột.

**Ranh giới BẮT BUỘC: chỉ cắt đuôi `.0` MỘT chữ số 0** (`^(-?[0-9]+)\.0$`). Nới thành `\.0+$` là
đụng thẳng §7: `"50.000"` (năm mươi nghìn theo quy ước VN) thành `50` — lệch 1000×, build xanh, test
cũ vẫn qua. `"1.0"` thì không thể là ngăn nghìn (ngăn nghìn luôn 3 chữ số) nên không nhập nhằng.

**Cũng phải loại trừ:**
- **cột tiền** — luật đọc tiền nằm trọn trong `helpers.ParseExcelMoney`, một nguồn sự thật;
- **cột chữ** (`name`, `code`, `desc`) — bản ghi đặt tên `"3.0"` là hợp lệ.

⚠ Chuẩn hoá phải áp **ở MỌI chỗ đọc cùng một ô**. `ImportServiceFromExcel` quét file 2 lượt (lượt 1
gom `importIDs` để batch-load, lượt 2 đọc từng ô); vá một lượt thì dòng vào nhánh UPDATE mà id lại
không có trong map batch-load → báo **"Không tồn tại"** cho bản ghi có thật.

Sự cố thật 2026-09-18 (1 khách prod, DB riêng của khách): file dịch vụ soạn trên Google Sheets, ô `id`
`"3.0"` → `ToInt64` = 0 → id không vào `importIDs` nên **câu query tìm bản ghi chưa từng chạy**, mà
dòng cũng không rơi về nhánh create (`rawID != ""`) → kẹt giữa hai nhánh, báo "Lỗi dòng 3: Không
tồn tại" rồi rollback cả file. Cùng file đó `service_type_id`/`emr_department_id` `"1.0"` bị đem đi
tra danh mục theo tên, và `status` `"1.0"` bị đòi nhập `"Đang dùng"`.

## 9. Cột ngày bị chủ động loại khỏi import — audit trước khi bật lại

2 handler có cột ngày ở file export nhưng KHÔNG import (đã có comment cảnh báo tại code):
- `hr/user/handler/handler.go` (`ImportUsers`) — "Ngày sinh"/birthday không có trong `userHeaderToCol`
- `loyalty/broker/handler/handler.go` (`ImportBroker`) — "Ngày tạo" map `_skip`

Cả 2 đang gọi `GetRows` KHÔNG `RawCellValue: true`. Bật lại cột ngày ở 2 chỗ này bắt buộc
đổi sang `RawCellValue: true` + `pkg/exceldate` (xem §6), không tự viết parser riêng.

## FE tương ứng

Xem `frontend/skill/skills/building-list-and-export-views/reference/fe-patterns-import-export.md` §5 — `ImportErrorDetailDialog` + `SKIP_ERROR_NOTIFICATION`.
