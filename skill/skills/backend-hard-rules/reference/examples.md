# Backend — ví dụ code ❌/✅

## Mục lục

- 1. Think Before Coding
- 2. Simplicity First
  - Example 2.1 — Over-abstraction cho rate limit
  - Example 2.2 — Speculative DTO
- 3. Surgical Changes
  - Example 3.1 — Fix tenant leak, đừng "improve" cả handler
  - Example 3.3 — Migrate decimal đừng thay logic xung quanh
- 4. Goal-Driven Execution
  - Example 4.1 — Plan phải verifiable
  - Example 4.2 — Reproduce trước khi fix: race condition appointment number
- 5. Timezone — FormatDate/FormatDateTime là timezone-unsafe
  - ❌ Cái LLM hay làm (copy helper cũ)
  - ✅ Cách đúng — Loc helpers

---


Code examples từ codebase ERP Cluvix. Stack: Go 1.25 + Gin + GORM + MySQL + Redis. Mỗi example: lỗi LLM thường mắc + cách đúng.

---

## 1. Think Before Coding

Yêu cầu mơ hồ ("thêm export", "tối ưu cho nhanh") → surface assumptions TRƯỚC khi code, không assume:

- **Scope tenant:** toàn bộ tenant hay filter status/branch/source? Giới hạn quyền per-user?
- **PII/permission:** phone, email, CCCD, field tiền — role nào được xem?
- **Format:** xlsx / csv / PDF — hỏi, đừng chọn hộ.
- **Volume:** tenant lớn ~50K row → sync request sẽ block; cần background job + link hay giới hạn max?
- **Naming/storage:** filename timestamp + tenant slug? S3 presigned URL hay stream?
- **"Chậm" phải đo trước khi tối ưu:** API (EXPLAIN, N+1, cache miss) vs bundle FE (`ng build --stats-json`) vs render (OnPush, track, virtual scroll) — hỏi user có data đo chưa, đo xong mới chọn hướng.

Trình bày các lựa chọn + effort ước lượng, đề xuất hướng đơn giản nhất, chờ chốt rồi mới implement.

---

## 2. Simplicity First

### Example 2.1 — Over-abstraction cho rate limit

**Yêu cầu:** "Thêm rate limit cho endpoint login: 10 fail/IP/5min"

**❌ Bloated (LLM hay viết)**

```go
type RateLimiter interface {
    Allow(key string) bool
    Reset(key string)
}

type RateLimitConfig struct {
    Backend   string        // "redis" | "memory"
    Algorithm string        // "fixed_window" | "sliding_window" | "token_bucket"
    Window    time.Duration
    Max       int
    KeyFunc   func(c *gin.Context) string
}

type RedisFixedWindowLimiter struct { /* 80 lines */ }
type RedisSlidingWindowLimiter struct { /* 100 lines */ }
type MemoryTokenBucketLimiter struct { /* 60 lines */ }

func NewRateLimiter(cfg RateLimitConfig) RateLimiter {
    switch cfg.Algorithm { /* factory */ }
}

func RateLimitMiddleware(limiter RateLimiter, ...) gin.HandlerFunc { ... }

// 400 dòng cho 1 use case "login fail counter"
```

**✅ Đủ dùng** (real từ `internal/modules/webapp/auth/core/handler/handler.go`)

```go
const (
    loginFailMax    = 10
    loginFailWindow = 5 * time.Minute
)

func loginFailKey(ip string) string {
    return cache.PrefixKey("auth:login_fail:" + ip)
}

func checkLoginRateLimit(ctx context.Context, ip string) bool {
    n, _ := cache.AppCache.Client().Get(ctx, loginFailKey(ip)).Int()
    return n < loginFailMax
}

func recordLoginFailure(ctx context.Context, ip string) {
    pipe := cache.AppCache.Client().Pipeline()
    pipe.Incr(ctx, loginFailKey(ip))
    pipe.Expire(ctx, loginFailKey(ip), loginFailWindow)
    pipe.Exec(ctx)
}

func clearLoginFailures(ctx context.Context, ip string) {
    cache.AppCache.Client().Del(ctx, loginFailKey(ip))
}
```

3 hàm thuần, 30 dòng. Khi có use case thứ 2 (vd: OTP rate limit) — refactor lúc đó.

---

### Example 2.2 — Speculative DTO

**Yêu cầu:** "API trả version field cho payment"

**❌ Bloated**

```go
type PaymentResponse struct { Data PaymentResponseData; Meta PaymentResponseMeta }
type PaymentResponseMeta struct {
    Version int64; UpdatedAt time.Time; UpdatedBy string
    Hash string       // for ETag — chưa cần
    APIVersion string // chưa support v2
}
type PaymentDTO struct { /* 40 field map từ model */ }
func toPaymentDTO(p *crmModels.Payment) PaymentDTO { /* 50 dòng convert */ }
```

**✅ Đơn giản**

```go
// Model đã có Version. Chỉ cần trả thẳng.
response.Updated(c, payment) // payment có Version trong JSON
```

Khi nào cần DTO? Khi muốn ẩn field nội bộ HOẶC version 2 API ra mắt. Lúc đó refactor.

---

## 3. Surgical Changes

### Example 3.1 — Fix tenant leak, đừng "improve" cả handler

**Yêu cầu:** "Fix tenant leak ở `GetCustomer` — đang dùng `db.DB` thay vì scope"

**❌ Drive-by refactor**

```diff
- func GetCustomer(c *gin.Context) {
-     id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
-     var cust crmModels.Customer
-     if err := db.DB.First(&cust, id).Error; err != nil {
-         response.NotFound(c)
-         return
-     }
-     response.Data(c, cust)
- }
+ // CustomerHandler — refactored to use struct injection
+ type CustomerHandler struct {
+     Svc *services.CustomerService
+     Logger *zap.Logger
+ }
+
+ func (h *CustomerHandler) Get(c *gin.Context) {
+     id, err := strconv.ParseUint(c.Param("id"), 10, 64)
+     if err != nil {
+         h.Logger.Warn("invalid id", zap.Error(err))
+         response.Validate(c, "ID không hợp lệ")
+         return
+     }
+     cust, err := h.Svc.GetByID(c, uint(id))
+     if err != nil {
+         h.Logger.Error("get customer", zap.Error(err))
+         response.ServerError(c, err)
+         return
+     }
+     response.Data(c, cust)
+ }
```

**Vấn đề:** Yêu cầu là fix tenant leak. LLM nhân tiện refactor sang struct injection, thêm logger, thay error response → diff khổng lồ, review khó, có thể introduce regression chỗ khác gọi `GetCustomer` với context cũ.

**✅ Surgical**

```diff
  func GetCustomer(c *gin.Context) {
      id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
      var cust crmModels.Customer
-     if err := db.DB.First(&cust, id).Error; err != nil {
+     if err := tenant.ScopedDBNullBranch(c, db.DB, "customer").First(&cust, id).Error; err != nil {
          response.NotFound(c)
          return
      }
      response.Data(c, cust)
  }
```

Đúng 1 dòng. Việc refactor sang struct injection — task riêng.

---
### Example 3.3 — Migrate decimal đừng thay logic xung quanh

**Yêu cầu:** "Đổi `Payment.Amount` từ float64 sang decimal.Decimal"

**❌ Drive-by**

```diff
  type Payment struct {
      ID         uint
-     Amount     float64
+     Amount     decimal.Decimal
      CustomerID uint
      Status     int
+     CreatedBy  uint   `json:"created_by"`
+     UpdatedBy  uint   `json:"updated_by"`
  }
+
+ // Bonus refactor: extract status to enum
+ type PaymentStatus int
+ const (
+     PaymentStatusPending PaymentStatus = iota + 1
+     PaymentStatusPaid
+     PaymentStatusRefunded
+ )
```

**✅ Surgical**

```diff
  type Payment struct {
      ID         uint
-     Amount     float64
+     Amount     decimal.Decimal
      CustomerID uint
      Status     int
  }
```

CreatedBy/UpdatedBy có ích — open issue riêng nếu thấy thiếu. Status enum — open issue khác .

---

## 4. Goal-Driven Execution

### Example 4.1 — Plan phải verifiable

❌ Vague: "1. Review code 2. Thêm version check 3. Test 4. Done" → code 500 dòng across 15 files, không có verification step.

✅ Khuôn mẫu plan — mỗi bước 1 verify cụ thể:

```
1. [Thay đổi cụ thể, file cụ thể] → Verify: `go build ./pkg/xxx` 0 error.
2. [Replace N sites — liệt kê file] → Verify: `grep -rn "<pattern>"` chỉ còn ở nơi mong đợi.
3. Build verify: `go build ./...` 0 error.
4. (Optional) Test manual: curl endpoint → expect status + message đúng.
```

Chỉ đổi đúng những site thuộc yêu cầu (Surgical principle). Trình plan → chờ OK rồi làm.

---

### Example 4.2 — Reproduce trước khi fix: race condition appointment number

**Yêu cầu:** "Lúc 2 user đặt lịch cùng lúc, appointment_number bị trùng"

**❌ Fix mò**

```go
// Tăng appointment_number bằng SELECT MAX + 1
var maxNum int
db.Model(&Appointment{}).Where("customer_id = ?", custID).
    Select("MAX(appointment_number)").Scan(&maxNum)
appt.AppointmentNumber = maxNum + 1
db.Create(&appt)
```

LLM đổi 1 dòng nhưng không reproduce → vẫn race (SELECT MAX + Create không atomic).

**✅ Reproduce → fix → verify**

```go
// Bước 1: Test reproduce
func TestAppointmentNumberRace(t *testing.T) {
    db := setupTestDB(t) // dockertest MySQL
    custID := seedCustomer(t, db)
    var wg sync.WaitGroup
    nums := make([]int, 10)
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(idx int) {
            defer wg.Done()
            appt := &Appointment{CustomerID: custID, AppointmentTypeID: 1}
            svc := NewAppointmentService(db)
            svc.Create(appt, 1)
            nums[idx] = int(appt.AppointmentNumber)
        }(i)
    }
    wg.Wait()
    seen := map[int]bool{}
    for _, n := range nums {
        if seen[n] {
            t.Fatalf("duplicate appointment_number %d", n)
        }
        seen[n] = true
    }
}
// Verify: chạy 10 lần, ít nhất 3-4 lần fail với MAX+1 logic.

// Bước 2: Fix với SELECT FOR UPDATE trong transaction
func (s *AppointmentService) Create(appt *Appointment, userID int) error {
    return s.DB.Transaction(func(tx *gorm.DB) error {
        var maxNum int
        tx.Raw(`
            SELECT COALESCE(MAX(appointment_number), 0) + 1
            FROM appointment
            WHERE customer_id = ? AND appointment_type_id = ?
            FOR UPDATE
        `, appt.CustomerID, appt.AppointmentTypeID).Scan(&maxNum)
        appt.AppointmentNumber = maxNum
        return tx.Create(appt).Error
    })
}

// Verify: chạy lại test 100 lần — không duplicate.
```

Goal-driven: bug đã reproduce trong test → fix có verifiable success.

---

## 5. Timezone — KHÔNG dùng `time.Local`

**Bug thực tế (ERP cũ):** helper `FormatDate`/`FormatDateTime` dùng `time.Unix(ts, 0).Format(...)` → `time.Local`, mà server cloud `time.Local = UTC` ⇒ giờ VN 12:00 hiển thị 05:00.

**Rule:**
- Format ngày/giờ trả về FE: `loc := tz.LocFromContext(c, h.DB)` rồi `time.Unix(ts, 0).In(loc).Format(...)`.
- KHÔNG `time.Local` / `time.Now().Location()`.
---

