# Gotchas GORM v2 — subquery/Session

## Mục lục

- GORM v2 — `.Session()` gọi SAU khi query đã bị mutate không hồi tố được (CRITICAL)
- GORM v2 — `.Order("")` KHÔNG xóa ORDER BY (DISTINCT subquery gotcha)
- GORM v2 — Session({}) clone GIỮ NGUYÊN Group/Order/Having (merge, không replace)

---


Tách từ `skill/skills/debugging-backend-queries/reference/patterns-gotchas-gorm.md` (topic: reuse 1 `*gorm.DB` đã build làm subquery/totals).

## GORM v2 — `.Session()` gọi SAU khi query đã bị mutate không hồi tố được (CRITICAL)

`pagination.Paginate(c, q, &items)` gọi `query.Offset(offset).Limit(perPage).Find(dest)` — mutate
**in-place** `q.Statement.Clauses` (thêm `LIMIT`/`OFFSET`) vì sau lần clone đầu tiên GORM chain có
`clone = 0`, các lệnh chain tiếp theo ghi thẳng lên cùng object thay vì tạo bản sao. Nếu tính
tổng (`SUM`) bằng cách wrap `q` làm subquery **SAU** khi gọi `Paginate`, subquery đó đã dính sẵn
`LIMIT`/`OFFSET` của trang hiện tại → tổng đổi theo trang, dù có bọc `.Session(&gorm.Session{})`.

**Lý do `.Session()` không cứu được:** `.Session(&gorm.Session{})` (không set `Context`/`PrepareStmt`/
`SkipHooks`) chỉ share lại cùng con trỏ `Statement` cũ (`tx.Statement = db.Statement`), không deep-clone.
Nó chỉ ngăn các mutation object VỀ SAU, không xoá được `LIMIT`/`OFFSET` đã bị ghi vào trước đó.

```go
// ❌ SAI — .Session() gọi sau Paginate, Statement đã dính LIMIT/OFFSET từ trước
result := pagination.Paginate(c, q, &items)
h.DB.Raw(`SELECT SUM(sub.x) FROM (?) AS sub`, q.Session(&gorm.Session{})).Scan(&totals) // vẫn theo trang

// ✅ ĐÚNG — snapshot tổng TRƯỚC khi gọi Paginate, lúc q còn sạch
h.DB.Raw(`SELECT SUM(sub.x) FROM (?) AS sub`, q.Session(&gorm.Session{})).Scan(&totals)
result := pagination.Paginate(c, q, &items)
```

Rule: mọi handler pattern "Paginate + totals subquery reuse `q`" → tính totals **TRƯỚC**
`pagination.Paginate`/`PaginateWithTotal`, không phải sau + wrap `.Session()`.

Precedent: `treatment_session_debts`, `treatment_session_period` — tổng công nợ đổi theo trang dù
đã có `.Session()`, do gọi sai thứ tự. Pattern đúng tham khảo `order_plan_debts`,
`revenue_doctor_consultant` (đã tính tổng trước `Paginate` từ đầu).

---

## GORM v2 — `.Order("")` KHÔNG xóa ORDER BY (DISTINCT subquery gotcha)

**Triệu chứng:** MySQL Error 3065 — `ORDER BY clause is not in SELECT list, references column not in DISTINCT`.

**Nguyên nhân:** GORM v2 `.Order("")` KHÔNG reset ORDER BY đã có — nó append thêm empty string vào clause. Khi lấy subQ từ query đã qua `ApplySort`, subQ mang theo ORDER BY từ ngoài, conflict với DISTINCT select list.

**Fix:** Build subQ TRƯỚC khi gọi `ApplySort`:
```go
// ✅ Build subQ trước — chưa có ORDER BY
subQ := q.Session(&gorm.Session{}).Select("DISTINCT x.payment_id, payment.amount").Offset(-1).Limit(-1)
var total decimal.Decimal
h.DB.Raw("SELECT COALESCE(SUM(s.amount), 0) FROM (?) s", subQ).Scan(&total)

// Sau đó mới ApplySort cho query chính
q = query.ApplySort(c, q, "payment.payment_of_date DESC, ...")
```

Nếu cần reset ORDER BY trên GORM v2: `db.Session(&gorm.Session{NewDB: true})` rồi build lại, hoặc dùng raw SQL cho subquery.

---

## GORM v2 — Session({}) clone GIỮ NGUYÊN Group/Order/Having (merge, không replace)

`q.Session(&gorm.Session{}).Select(...)` chỉ replace Select — `Group`/`Order`/`Having` của `q`
gốc vẫn còn nguyên trong bản clone (GORM merge/append các clause này, không như Select).
Nếu dùng clone làm subquery `SELECT <1 cột id>` cho `IN (?)`, HAVING/GROUP BY cũ (tham chiếu
alias chỉ tồn tại ở Select gốc) sẽ leak vào và gây lỗi "Unknown column ... in having/group clause".

```go
// ✅ Xoá thủ công qua Clauses map trước khi Select 1 cột làm subquery
clone := idQuery.Session(&gorm.Session{})
delete(clone.Statement.Clauses, "GROUP BY") // GORM lưu chung Group + Having dưới key "GROUP BY"
idSubQ := clone.Select(idCol)
```

Áp dụng cho mọi trường hợp reuse 1 query đã có Group/Having làm subquery con.
