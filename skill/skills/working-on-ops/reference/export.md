# Export

## Mục lục

- Cơ chế
- Infrastructure
- Pattern chuẩn
  - Export dùng làm template import (export → sửa → import lại)
- Export functions (85 — nhóm chính, không đầy đủ)
  - Tier 1: Đã có ✅
  - Tier 2: CRM exports ✅
  - Tier 3: Med ✅
  - Tier 4: Multi-sheet ✅
  - Tier 5: Med Reports ✅ (15 functions)
- Helpers
- Phone visibility trong export (CRITICAL)
- colDef pattern — export có column filter (CRITICAL khi FE có selectedColumns)
  - Core columns — cột FE luôn hiện (không toggle) PHẢI luôn có trong export (CRITICAL)
- Tránh correlated subquery / N+1 trong export (CRITICAL)
- history_export — ghi lịch sử mỗi lần export
- Timestamp "đầu ngày" trong export (CRITICAL)

---


## Cơ chế
- FE gọi `?_export=1` trên list endpoint
- List handler tách `BuildXxxQuery(c) *gorm.DB` (public), khi `_export=1` gọi export handler
- < 10k rows → Excel, ≥ 10k → CSV
- Batch 5000 rows/batch (Offset/Limit loop) tránh OOM
- **CRITICAL — `.Offset()/.Limit()` bị GORM bỏ qua khi query gốc là `db.Raw(...)`** (vd export UNION): chain `.Session().Offset(o).Limit(n).Find(&rows)` trả về **toàn bộ** rows, không cắt trang → lặp data mỗi batch khi total > BatchSize. Fix: dùng `fetchBatch()` (subquery wrap `SELECT * FROM (?) AS batch_q LIMIT ? OFFSET ?`) — hoạt động đúng cho cả query builder lẫn Raw, cùng kỹ thuật với COUNT query đã dùng.
- File tự xóa sau 30 phút
- **Thứ tự export = thứ tự list, do FE gửi `sort` (CRITICAL — bug 2026-08-13)**: export dùng CHUNG query với list nên ORDER BY do `query.ApplySort` quyết định. FE trước đây dựng param export thủ công `{...defaultSearch(), ...search(), ...requiredSearch()}` — thiếu `sort` → BE rơi về defaultOrder (thường `id DESC`) → file export ra khác hẳn bảng user đang nhìn. Đã sửa: FE dùng `BaseListV2.exportParams()` (giữ `sort`, bỏ `page`/`per-page`) — **thêm màn export mới KHÔNG được tự dựng lại object param**. Kèm theo: field FE sort được PHẢI có trong whitelist `ApplySort` của handler, không thì BE fallback âm thầm (có log `SortRejected`).
- **Sort không unique phải có tiebreaker**: `fetchBatch` cắt trang bằng `LIMIT/OFFSET`, nên ORDER BY theo cột không unique (`appointment_date`, `created_day`, `confirm_time`...) cho thứ tự không xác định giữa các row bằng nhau → row lặp/mất ở ranh giới batch 5000. `ApplySort` nay tự nối `<bảng gốc>.id DESC` (suy từ prefix của defaultOrder). ⚠ Query `GROUP BY` KHÔNG theo pk bảng gốc sẽ vỡ vì `ONLY_FULL_GROUP_BY` (error 1055) — xem doc comment `ApplySort` trong `pkg/query/filter.go`.
- Response: `c.JSON(http.StatusOK, url)` — trả JSON string URL trực tiếp, KHÔNG wrap object; FE `window.location.href = res`

## Infrastructure
- `pkg/export/export.go` — Generic engine: Excel/CSV auto-detect, batch processing, sanitize formula injection
- `internal/modules/webapp/export/` — flat package, 85 functions (free-fn `ExportXxx(c, query)`, không có sub-resource). Đếm chính xác: `grep -c '^func Export' internal/modules/webapp/export/*.go`. `DownloadURL` set qua `exportHandlers.DownloadURL field(url)` từ bootstrap.

## Pattern chuẩn
```
Export handler nhận query đã build từ list handler
→ Thêm SELECT + JOINs để lấy flat data
→ Define headers + row mapper
→ Gọi export.Export()
```

**KHÔNG tự viết `excelize.NewFile()` + `q.Find(&items)` trực tiếp** cho export theo dòng giao dịch (log nhập/xuất/dispense...) — bounded ảo do "ít data lúc viết code" nhưng tăng dần theo thời gian, sẽ OOM/chậm khi table lớn. Luôn qua `doExport`/`export.Export()` để có batch + CSV fallback tự động. Chỉ OK tự viết excelize khi query có `GROUP BY` theo catalog nhỏ (thuốc/NCC/nhân viên — bounded tự nhiên).

### Export dùng làm template import (export → sửa → import lại)

- Khai `export.Config.FieldKeys` (tên cột DB): engine ghi thành **dòng header thứ 2**; importer map theo dòng 2, fallback nhãn dòng 1 cho file cũ.
- Importer PHẢI gọi `export.Unsanitize()` từng ô — `sanitize()` chèn `'` vào đầu chuỗi bắt đầu bằng `= + - @ \t \r`, không bóc thì `- Combo A` quay lại DB thành `'- Combo A`, mỗi vòng chồng thêm 1 dấu.
- Chiều ngược lại: mọi ô chứa dữ liệu người dùng nhập phải qua `export.SanitizeCell` — kể cả sheet phụ (danh mục tra cứu), không chỉ sheet dữ liệu của `export.Export`.
- Cột tiền **nullable** export ra `nil` (ô trống), KHÔNG `toFloat` (trả 0) — export rồi import lại là NULL thành 0 thật trong DB.
- `FieldKeys` lệch độ dài `Headers` → `validateConfig` reject ngay; cả 3 nhánh ghi (Excel, CSV, multi-sheet) dùng chung `headerRows()`.
- **CRITICAL** — thêm validate cột (enum/range/độ dài) ở importer PHẢI nhận được **mọi giá trị đang tồn tại trong DB**, kể cả giá trị không có trong danh sách chọn của form (`DEFAULT 0`, dữ liệu nhập từ hệ thống cũ). File export ghi thẳng giá trị DB nên validator hẹp hơn dữ liệu = user export rồi import lại mà không sửa gì vẫn rollback cả file. Test khoá: duyệt mọi cột enum, khẳng định `0` import lại được.
- **File mẫu import LUÔN là file tĩnh ở `frontend/src/assets/download/` — BE KHÔNG có endpoint phát file mẫu** (user chốt 2026-09-08, đã gỡ `GET /config/service/import-template`). Sinh bằng `cd backend && go run ./cmd/gen-template -only=<x>` rồi commit file `.xlsx` vào repo frontend; FE tải bằng thẻ `<a href="/assets/download/...">`, KHÔNG gọi API.
- Template sinh từ chính colDef export (`XxxExportHeaders/FieldKeys`) nên không lệch cột với file export; sheet dữ liệu chỉ có 2 dòng header và PHẢI là sheet ĐẦU TIÊN (importer đọc `GetSheetName(0)`) — dòng ví dụ/ghi chú là dòng import thật, không để trong đó.
- `xlsxtemplate.Spec.Catalog` chỉ nhận danh mục CỐ ĐỊNH của hệ thống (enum: trạng thái, nhóm tuổi, loại hoa hồng...). Danh mục theo tenant (id khoa/nhóm DV/đơn vị/thương hiệu/thuế khác nhau mỗi DB) KHÔNG vào file mẫu — nó chỉ đi kèm **file export** dưới dạng sheet tra cứu (`ServiceCatalogColumns` → `ServiceCatalogSheet`), và sheet hướng dẫn của file mẫu phải nói rõ user tra id bằng cách bấm Export. Xem `export/service_template.go`: cờ `serviceCatalogDef.Static` phân đôi hai đường này.
- Dropdown Excel phải trỏ **vùng ô** của sheet danh mục (`Danh_muc!$A$2:$A$n`), KHÔNG nhúng danh sách vào ô — Excel giới hạn ~255 ký tự, vượt thì bỏ luôn dropdown, không báo gì. Danh mục rỗng → KHÔNG gắn dropdown (trỏ vùng trống = Excel chặn mọi giá trị).

## Export functions (85 — nhóm chính, không đầy đủ)

### Tier 1: Đã có ✅
user, service, unit

### Tier 2: CRM exports ✅
appointment, broker (3), customer (7), coupon (3), order (5)

### Tier 3: Med ✅
med_medicine

### Tier 4: Multi-sheet ✅
med_stock_import (2 sheets + 5 catalog), med_purchase_order (2 sheets + 5 catalog)

### Tier 5: Med Reports ✅ (15 functions)
stock_report, expiry, low_stock, revenue, profit, top_selling, controlled_drug, supplier, by_staff, book_import, book_dispensed, book_controlled, return_destroy, inventory_audit, batch_tracking

## Helpers
`fmtUnixDate`, `fmtUnixDateTime`, `toFloat`, `toInt`, `stripHTML`, `fmtJSON`, `writeRow`, `fmtUnixDateSlash`

## Phone visibility trong export (CRITICAL)

Export file chứa `phone`/`other_phone` → **bắt buộc** dùng `CanSeePhoneExport`, **KHÔNG** dùng `CanSeePhone`:

```go
showPhone := phoneutil.CanSeePhoneExport(c, q.Session(&gorm.Session{NewDB: true}))
// Trong row mapper:
maskPhoneExport(showPhone, row["phone"])  // helper local → phoneutil.MaskPhone
```

**Tại sao khác `CanSeePhone`:**
- `CanSeePhone` (option 14) — list API: ẩn 4 số cuối với non-BOD khi ON
- `CanSeePhoneExport` (option 20) — export Excel: OFF=chỉ BGĐ thấy SĐT, ON=tất cả thấy

Hai hàm độc lập, không thay thế nhau. Chi tiết option 20 + danh sách callsite đã apply / export files không cần mask → Xem `skill/skills/managing-crm-options/reference/option-crm-options-p2.md` §Option 20.

## colDef pattern — export có column filter (CRITICAL khi FE có selectedColumns)

Khi list có `selectedColumns` và cần FE chọn cột export, dùng `colDef` slice thay hardcode.

```go
type receiptCol struct {
    key     string                                          // khớp với FE selectedColumns value
    header  string
    extract func(row map[string]interface{}) interface{}
}

var receiptCols = []receiptCol{
    {"customer_id", "Khách hàng", func(r map[string]interface{}) interface{} { return r["customer_fullname"] }},
    // ... toàn bộ cột
}

func ExportXxx(c *gin.Context, q *gorm.DB) {
    // 1. Build query với tất cả SELECT + JOIN cần thiết (luôn SELECT all — filter ở mapper)
    eq := q.Session(&gorm.Session{}).Select(`...all fields...`).Joins(...)

    // 2. Filter colDef theo ?columns= param
    cols := receiptCols
    if colParam := c.Query("columns"); colParam != "" {
        selected := map[string]bool{}
        for _, k := range strings.Split(colParam, ",") {
            selected[strings.TrimSpace(k)] = true
        }
        var filtered []receiptCol
        for _, col := range receiptCols {
            if selected[col.key] { filtered = append(filtered, col) }
        }
        if len(filtered) > 0 { cols = filtered }
    }

    // 3. Build headers + mapper động
    headers := make([]string, len(cols))
    for i, col := range cols { headers[i] = col.header }

    url, err := doExport(c, eq, export.Config{
        FileName: "...", Headers: headers,
        ExportDir: "./storage/export/", DownloadURL: DownloadURL,
    }, func(row map[string]interface{}) []interface{} {
        data := make([]interface{}, len(cols))
        for i, col := range cols { data[i] = col.extract(row) }
        return data
    })
    ...
}
```

**Rules:**
- SELECT luôn lấy ALL fields — không filter SELECT theo columns param (tránh alias mismatch)
- `colDef.key` PHẢI khớp 1-1 với `value` trong FE `displayCols`/`availableColumns`
- **Gotcha — cột trắng âm thầm:** mỗi `r["x"]` trong `extract` PHẢI có trong SELECT (bare col hoặc `as x`); thiếu → cột export rỗng mọi dòng, KHÔNG panic/lỗi. Sửa/thêm cột colDef → đối chiếu lại SELECT.
- **colDef dùng chung nhiều list** (vd `appointmentColsFor` cho in-menu + doctor): thêm cột mới **append CUỐI** slice để không đổi thứ tự cột "all"-mode của list khác; chèn giữa → phải sửa FE `displayCols` của mọi list dùng chung.
- Fallback khi không có `columns` param → export all (backward compatible)
- Cần `import "strings"` cho `strings.Split`

**Ví dụ thực tế:** `ExportPaymentReceipts` trong `internal/modules/webapp/export/payment.go`

### Core columns — cột FE luôn hiện (không toggle) PHẢI luôn có trong export (CRITICAL)

Nhiều list có cột "khung" render cứng trong HTML — không bọc `@if (isColVisible('key')())` — vì đó
là số liệu cốt lõi của report (khách hàng, số tiền, công nợ...) nên cố tình không cho ẩn. Cột này
**không** nằm trong FE `displayCols`/`selectedColumns` (vì không cần toggle).

**Gotcha đã xảy ra thật (2026-07-14, 3 report công nợ kế toán):** khi mode=`'selected'`,
FE gửi `?columns=` chỉ chứa key trong `selectedColumns` — filter phẳng theo key ở BE loại bỏ
luôn cả cột core (vì key của nó không nằm trong `selectedColumns`, do FE chưa từng đưa vào
displayCols). Kết quả: file export thiếu cột khách hàng/số tiền dù bảng UI luôn hiển thị —
sai lệch nghiêm trọng giữa cái đang xem và cái export ra, không lỗi/không cảnh báo gì.

**Fix — filter nhận thêm `coreKeys`, luôn ép các key này có mặt bất kể `columns` param:**

```go
// coreKeys: cột UI hiển thị luôn (không có trong FE displayCols toggle, không bọc
// isColVisible trong template) — LUÔN giữ trong export bất kể user chọn gì.
func filterExportCols(all []xxxCol, colParam string, coreKeys ...string) []xxxCol {
    if colParam == "" {
        return all
    }
    selected := map[string]bool{}
    for _, k := range strings.Split(colParam, ",") {
        selected[strings.TrimSpace(k)] = true
    }
    for _, k := range coreKeys {
        selected[k] = true
    }
    var filtered []xxxCol
    for _, col := range all {
        if selected[col.key] {
            filtered = append(filtered, col)
        }
    }
    if len(filtered) == 0 {
        return all
    }
    return filtered
}

// callsite:
cols := filterExportCols(xxxColsFor(...), c.Query("columns"), "fullname", "totalEnd", "debtByUnitPrice")
```

**KHÔNG** đưa cột core vào `displayCols`/`selectedColumns` FE để "fix" — sẽ cho user toggle-ẩn được
trong popup `app-column-toggle` trong khi bảng vẫn cứ hiện cứng (popup nói dối, mất đồng bộ UI/state).
Cột core phải ở nguyên trạng "không toggle" cả 2 phía — FE giữ hardcode không `isColVisible`, BE ép
luôn include qua `coreKeys`.

**Checklist khi thêm colDef mới có `?columns=` filter:** đối chiếu HTML — cột nào **không** bọc
`isColVisible`/`visibleColumns()` mà **có** colDef key tương ứng → thêm vào `coreKeys` ngay từ đầu,
đừng đợi phát hiện qua bug report.

**Ví dụ thực tế:** `treatmentDebtsColsFor`/`installmentDebtColsFor` (module công nợ ERP, đã bỏ khỏi CRM standalone).

**Hệ quả FE:** sau khi ép `coreKeys`, số cột thực export ở mode `'selected'` > `selectedCount`
hiển thị trên dialog — label `ExportOptionsDialog` (`utilities.export_options_dialog.selected_label`/
`all_label`) KHÔNG dùng `{{count}}` để tránh lệch số (đã bỏ ở cả `public/` + `src/` assets/i18n).

## Tránh correlated subquery / N+1 trong export (CRITICAL)

KHÔNG dùng `(SELECT ... WHERE x = outer.col)` hay JOIN JSON_TABLE lặp lại theo từng dòng
trong Select() của export — chạy lại theo mỗi dòng kết quả, chậm khi export nhiều dòng.

**Fix — preload 1 lần, scope theo IN-subquery cùng filter với query chính:**
- Aggregate theo id (SUM/MAX...): `loadSumByFilteredIDs(idQuery, idCol, table, groupCol, sumCol)`
- Tên master data (id → fullname), toàn bảng: `loadNameMap(db, table, idCol, nameCol)`
- Tên user từ JSON array column (`listRevenues`/`listDoctors`/`listDirectSales`): `resolveUserNames(row["colJSON"], nameByID)`
- Tên gộp nhiều user theo group (kỹ thuật viên/phụ tá...): `loadUserNamesByGroupID(idQuery, idCol, table, groupCol, userIDCol)`
- "Bản ghi gần nhất" (greatest-n-per-group, vd lịch hẹn/điều trị gần nhất): JOIN derived `MAX(id)`/`MAX(created_time)` — xem `loadLastTreatmentByServiceUsedID`, `loadLatestAppointmentDateByCustomerID`

Helpers dùng chung: `internal/modules/webapp/export/aggregate.go`.

**Gotcha khi build idQuery subquery**: query gốc (`q`) có thể mang theo `Group`/`Having` từ
handler (vd `.Having("debtByUnitPrice != 0 ...")` chỉ hợp lệ trong Select của hàm export, không
tồn tại khi subquery chỉ Select 1 cột id) → dùng `cleanIDSubQuery(idQuery, idCol)` (tự xoá
`GROUP BY`/`HAVING` trước khi Select id), KHÔNG tự `.Session(&gorm.Session{}).Select(idCol)` trực tiếp.

**422 khi export không phải luôn là bug**: `pkg/export.Export()` trả lỗi
`"Không có dữ liệu để export"` khi query 0 dòng → `response.HandleError` map thành 422. Check
response body trước khi coi là lỗi cột/query.

## history_export — ghi lịch sử mỗi lần export

`doExport()` trong `internal/modules/webapp/export/history.go` tự động ghi vào `history_export` sau mỗi export thành công.

- `DB` set 1 lần qua `exportHandlers.SetDB(db.DB)` trong bootstrap
- Multi-sheet exports gọi `recordHistory(c, position, count)` trực tiếp sau `f.SaveAs`
- Table: `history_export` (positionExport, quantityRecord, ip_address, userAgent, createdBy, createdAt)
- Đọc lịch sử qua `GET /history/history_export`

Callsite `CanSeePhoneExport` đã apply + export files không cần mask → Xem `skill/skills/managing-crm-options/reference/option-crm-options-p2.md` §Option 20.

## Timestamp "đầu ngày" trong export (CRITICAL)

Export filter theo ngày hoặc tính `daysLeft` phải dùng company-TZ midnight (`tz.StartOfDay(loc)` với `loc := tz.LocFromContext(c, db)`), KHÔNG dùng `time.Now().Truncate(24h)` (UTC midnight) hay `Unix()/86400`.

→ Xem `skill/skills/debugging-backend-queries/reference/patterns-gotchas.md` §Timezone — Day Boundary.
