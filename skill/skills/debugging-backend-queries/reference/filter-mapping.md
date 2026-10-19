# Filter Query Mapping

## Mục lục

- Quy trình khi được yêu cầu "làm tương tự với [Component]"
  - Bước 1 — Đọc FE initFilter()
  - Bước 2 — Đọc BE handler/query
  - Bước 3 — So sánh, tìm field FE có nhưng BE thiếu
  - Bước 4 — Fix BE nếu thiếu
  - Bước 5 — Fix FE
- Quy tắc getDefaultFilterValues
- Operators của ApplyFilter (pkg/query/filter.go)
- Custom handler (KHÔNG dùng ApplyFilter) — đọc đúng key
- Filters đặc biệt (custom, không dùng ApplyFilter)
- JSON_TABLE — unnest JSON array để GROUP BY / SUM
- Lưu ý

---


## Quy trình khi được yêu cầu "làm tương tự với [Component]"

### Bước 1 — Đọc FE initFilter()
Liệt kê từng field:
- `field_name` → kiểu (`DateRange` / `multiselect 'in'` / `select 'equals'` / `keyword 'contains'`)
- Ghi chú `defaultSearch` nếu có

### Bước 2 — Đọc BE handler/query
Liệt kê filter đang có:
- `ApplyFilter(field, column)`
- Custom: JSON_CONTAINS, JOIN lookup, keyword

### Bước 3 — So sánh, tìm field FE có nhưng BE thiếu

### Bước 4 — Fix BE nếu thiếu
```go
q = query.ApplyFilter(c, q, "field_name", "table.column")
```
Sau đó `go build ./...` verify.

### Bước 5 — Fix FE

**5a.** Sửa `initFilter()` — multiselect `'in'` phải `[]`, không phải `null`

**5b.** Thêm pattern:
```typescript
clearFilter() {
  // Nếu có defaultSearch thì reset về mặc định
  // this.defaultSearch.set({ 'filter[field][gte]': ..., 'filter[field][lte]': ... });
  this.search.set({});
  this.rfFilterModel.reset(this.getDefaultFilterValues(), { emitEvent: false });
  this.dt?.clearFilterValues();
  this.getList();
}

private getDefaultFilterValues() {
  return {
    _keyword: null,       // keyword → null
    field_multi: [],      // multiselect 'in' → []
    field_single: null,   // select 'equals' → null
    field_date: null,     // DateRange không default → null
    field_date_default: [ // DateRange có default
      this.timezoneService.monthStart(),
      this.timezoneService.todayStart(),
    ],
  };
}
```

**5c.** Xóa `override resetPage()` nếu có — `BaseList` đã có sẵn, không cần override.

**5d.** Thêm button **Xóa bộ lọc** vào HTML, sau button **Bộ lọc**:
```html
<p-button
  (click)="clearFilter()"
  icon="pi pi-filter-slash"
  pTooltip="Xoá bộ lọc"
  severity="warn"
  tooltipPosition="top"
  type="button"
></p-button>
```

## Quy tắc getDefaultFilterValues
- `'in'` → `[]`
- `'between'` không default → `null`
- `'between'` có default → `[start, end]` khớp `defaultSearch`
- `'contains'` → `null`
- `'equals'` → `null`

## Operators của ApplyFilter (pkg/query/filter.go)
| FE gửi | Query param | SQL |
|---|---|---|
| Single | `filter[field]=val` | `= ?` |
| Array/IN | `filter[field][0]=a&[1]=b` | `IN ?` |
| Range gte | `filter[field][gte]=val` | `>= ?` |
| Range lte | `filter[field][lte]=val` | `<= ?` |
| Like | `filter[field][like]=val` | `LIKE %val%` |

## Custom handler (KHÔNG dùng ApplyFilter) — đọc đúng key

FE `buildFilter` serialize khác với `ApplyFilter` expects:

| matchMode FE | Key FE gửi | BE đọc đúng |
|---|---|---|
| `'equals'` | `filter[field][eq]=val` | `c.Query("filter[field][eq]")` |
| `'in'` | `filter[field][0]=a&filter[field][1]=b` | loop `fmt.Sprintf("filter[field][%d]", i)` |
| `'custom'` (field `_`) | `_field=val` | `c.Query("_field")` |

Cơ chế thứ 3 (ngoài `ApplyFilter`/`c.Query` custom): Gin **struct-tag binding** — field `[gte]`/`[lte]` khai `form:"filter[x][gte]" binding:"omitempty"` trên struct `ListOptions` rồi `validation.BindQuery(c, &opts)`, KHÔNG có lời gọi hàm nào cả. Grep `ApplyFilter(`/`c.Query(` ra 0 với dạng này — khảo sát "field có sống không" phải đọc luôn struct `ListOptions`/`dto.go` của handler, không chỉ grep lời gọi hàm.
```go
type ListOptions struct {
    DateStart *int64 `form:"filter[created_day][gte]" binding:"omitempty"`
    DateEnd   *int64 `form:"filter[created_day][lte]" binding:"omitempty"`
}
```

```go
// ✅ equals
if v := c.Query("filter[status][eq]"); v != "" { ... }

// ✅ in (indexed array)
var vals []string
for i := 0; i < 10; i++ {
    if v := c.Query(fmt.Sprintf("filter[sending_mode][%d]", i)); v != "" {
        vals = append(vals, v)
    }
}

// ❌ bug phổ biến
c.Query("filter[status]")           // trả "" — FE không gửi key này cho equals
c.QueryArray("filter[field][]")     // trả [] — FE gửi [0],[1] không phải []
```

## Filters đặc biệt (custom, không dùng ApplyFilter)
| FE field | BE xử lý |
|---|---|
| `_keyword` | `ApplyKeywordFilter(c, q, "table.col1", ...)` |
| `_groupSource` | `GetSourceIdsByGroupSource` → `WHERE source_id IN ?` |
| `_groupLocal` | `GetProvinceIdsByGroupLocal` → `WHERE province_id IN ?` |
| `_groupCountry` | `GetCountryIdsByGroupCountry` → `WHERE country_id IN ?` |
| `_department` | `JSON_CONTAINS(user_profile.departmentList, ?)` |
| `_listRevenues` | `JSON_CONTAINS(order_service_used.listRevenues, CAST(? AS JSON), '$')` — field trên `order_service_used`, KHÔNG phải `payment` |
| `_listDoctors` | `JSON_CONTAINS(order_service_used.listDoctors, CAST(? AS JSON), '$')` — tương tự |
| `_listDirectSales` | `JSON_CONTAINS(order_service_used.listDirectSales, CAST(? AS JSON), '$')` — tương tự |
| `_relatedUser` | `JSON_OVERLAPS(reminder.relatedUser, ?)` |

## JSON_TABLE — unnest JSON array để GROUP BY / SUM

Dùng khi cần aggregation trên JSON array column (`listRevenues`, `listDoctors`, `listDirectSales`). `JSON_CONTAINS` chỉ dùng cho filter/WHERE.

```sql
-- GROUP BY user trong listRevenues, SUM doanh số
SELECT j.user_id, SUM(osu.totalEnd) AS doanhSo
FROM order_service_used osu
JOIN order_treatment_plan o ON o.id = osu.treatment_plan_id
JOIN service s ON s.id = osu.service_id
JOIN JSON_TABLE(osu.listRevenues, '$[*]' COLUMNS(user_id INT PATH '$')) j
WHERE j.user_id IN ? AND s.serviceCategory_id != ? AND osu.created_day BETWEEN ? AND ?
GROUP BY j.user_id
```

MySQL 8.0+ only. `JSON_CONTAINS` cho filter, `JSON_TABLE` cho aggregation — không thay thế nhau.

## Lưu ý
- `_status` → `customer.status` (filter trên bảng JOIN)
- `_gender` → `customer.gender`
- `sourceCustomer_id` → `customer.source_id` (cần JOIN customer)
- Field prefix `_` thường filter trên bảng JOIN, không phải bảng chính
