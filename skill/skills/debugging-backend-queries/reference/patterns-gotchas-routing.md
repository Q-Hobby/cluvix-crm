# Gotchas Routing/HTTP layer

## Mục lục

- Query Param vs JSON Body — Type Mismatch (CRITICAL)
- Pagination handler — `c.JSON` vs `response.Data` (CRITICAL)
- Gin: route mới share tiền tố với `:param` có sẵn PHẢI cùng tên wildcard

---


## Query Param vs JSON Body — Type Mismatch (CRITICAL)

`c.Query()` và `c.Param()` **luôn trả `string`**. `ToInt64(string)` hit `default: return 0` — sai âm thầm.

```go
// ❌ NEVER
id := ToInt64(c.Query("branchID"))   // "3" → 0 → query sai

// ✅ ALWAYS
id, _ := strconv.ParseInt(c.Query("branchID"), 10, 64)
```

**JSON body** — Gin decode JSON number → `float64` → `ToInt64` OK:
```go
branchID := body["branch_id"]   // float64(3) → ToInt64 = 3 ✅
branchID := c.Query("branchID") // "3"       → ToInt64 = 0 ❌
```

**Rule:**
- Int từ query param → `strconv.ParseInt` / `strconv.ParseFloat`
- `ToInt64` chỉ dùng cho JSON body hoặc DB scan


**Phát hiện "ô trống" trong payload map — KHÔNG lọc bằng helper type-assert string (CRITICAL)**

`strFromAny(v)` (và mọi helper `v.(string)` rồi trả `""`) trả `""` cho **MỌI** giá trị không phải string ⇒ JSON number (`float64`) bị coi là rỗng.

```go
// ❌ NEVER — body JSON {"tax_rate_id": 5} → float64 → strFromAny = "" → ghi NULL, vẫn HTTP 200
if raw == nil || strFromAny(raw) == "" || helpers.ToInt64(raw) <= 0 { updates[k] = nil }

// ✅ ALWAYS — ToInt64 phủ nil→0, ""→0, "5"→5, float64(5)→5
if helpers.ToInt64(raw) <= 0 { updates[k] = nil }
```

Precedent: `config/service` `tax_rate_id` — UI báo "Cập nhật thành công" mà DB không đổi. Fix 2026-08-26.

---

## Pagination handler — `c.JSON` vs `response.Data` (CRITICAL)

`response.Data(c, result)` bọc thêm lớp `{data: {...}}` — dùng cho single-item response.
`pagination.Paginate()` trả `{items, _meta}` — FE `BaseApiService.list()` expects format đó TRỰC TIẾP, không có wrapper.

```go
// ❌ NEVER — FE nhận {data:{items,_meta}}, response.items = undefined → list rỗng
result := pagination.Paginate(c, q, &items)
response.Data(c, result)

// ✅ ALWAYS cho list handler
result := pagination.Paginate(c, q, &items)
c.JSON(http.StatusOK, result)
```

Rule: CUD handler → `response.Created/Updated/Deleted/Data(c, item)`
      List handler → `c.JSON(http.StatusOK, result)`

---

## Gin: route mới share tiền tố với `:param` có sẵn PHẢI cùng tên wildcard

Route mới đứng cùng vị trí cây route với 1 route `:param` đã đăng ký trong CÙNG group (bất kỳ HTTP method nào) bắt buộc dùng LẠI đúng tên param đó — dù ý nghĩa dữ liệu khác nhau. Gin panic "wildcard conflicts with existing wildcard" nếu đặt tên khác.

```go
// Đã có: r.GET("/:token", h.GetQuiz); r.POST("/:token", h.SubmitQuiz)

// ❌ Panic khi khởi động — tên wildcard khác ":token" cùng vị trí
r.POST("/:quiz_id/session", h.CreateSession)

// ✅ Phải tái dùng tên "token", dù giá trị thực là quiz_id
r.POST("/:token/session", h.CreateSession)
// trong handler: quizID := c.Param("token") — comment rõ ý nghĩa thực khác tên
```

Route tĩnh (`/history`) không bị ảnh hưởng — gin ưu tiên match static trước wildcard cùng cấp, không cần đổi tên. Precedent: `client/customer-b/route.go` (`CreateSession` mượn tên `:token` cho `quiz_id`), fix 2026-07-10.
