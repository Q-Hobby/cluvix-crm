# pkg/repository — Generic Base DAO

**Status (2026-05-16):** ⏸️ **Reserved for new modules only.** Pilot rollout vào module CRM hiện có đã pause sau audit — handlers hiện tại return `[]map[string]interface{}` với join customer (vd `reminder`, `note`, `customer_warranty_card`), không khớp shape `[]Entity` typed mà `BaseRepo[T]` thiết kế cho. Migrate retrofit chỉ tiết kiệm 5-10 dòng/handler ở phần Delete/Update, không bù được chi phí indirection. Chi tiết: `backend/plan/02-base-repo-interface.md` section "Lesson learned — pause".

Pkg vẫn hoạt động đúng — `BaseInterface[T]` đã thêm để cho phép mock trong unit test. Khi module mới được viết theo Plan 03 (layered layout) + Plan 04 (DTO) với entity typed return, dùng `BaseRepo` từ đầu sẽ tự nhiên phát huy giá trị.

## Khi nào dùng `BaseRepo[T]`

✅ **Nên dùng cho:**
- **Module mới** viết theo layered layout (`internal/modules/<domain>/<resource>/`) với entity typed return, DTO body.
- Endpoint trả `[]Entity` thuần (không join + attach map cho FE).
- Endpoint cần optimistic lock (`UpdateWithVersion`).

❌ **KHÔNG dùng để retrofit module CRM hiện có:**
- List/Get hiện trả `[]map[string]interface{}` với joined fields → BaseRepo typed return sẽ vỡ FE response shape.
- Create/Update hiện có business hook (codegen, last_contact_date update, html.UnescapeString) → không nằm trong `BaseRepo.Create()`.
- Query phức tạp: joins nhiều bảng, subquery, raw SQL → viết handler bằng `tenant.ScopedDBNullBranch` trực tiếp.
- Business-flavored read queries (vd appointment conflict check) → dùng pattern Repository Reader hiện có ở `internal/repository/<module>/`.

`BaseRepo` không thay thế `internal/repository/<module>/` — chúng giải quyết bài toán khác:
- `BaseRepo` = generic CRUD with tenant-safe defaults.
- `internal/repository/` = domain-specific read methods cho unit test service hot-spot.

## Quick start

```go
// 1. Khai báo repo trong handler module
type LeadRepo struct {
    *repository.BaseRepo[crmModels.CustomerLead]
}

func NewLeadRepo(db *gorm.DB) *LeadRepo {
    return &LeadRepo{repository.New[crmModels.CustomerLead](db, "customer_lead")}
}

// 2. Embed vào Handler.Deps
type Handler struct {
    Deps
    LeadRepo *LeadRepo
}

// 3. Dùng trong handler
func (h *Handler) Get(c *gin.Context) {
    id, _ := helpers.ParseID(c)
    item, err := h.LeadRepo.FindByID(c, id, nil)
    if err != nil {
        response.HandleError(c, err) // ErrNotFound → 404
        return
    }
    response.Data(c, item)
}
```

## Tenant safety contract

| Method | Tenant scope? | Notes |
|---|---|---|
| `FindByID` | ✅ Default (`ScopedDBNullBranch`) | Out-of-scope → `ErrNotFound` (không leak existence) |
| `List` | ✅ Always | `mods` áp thêm sau scope |
| `Count` | ✅ Always | Same |
| `Create` | ❌ Không scope | Caller PHẢI set `company_id`/`branch_id` + gọi `companyAllowed` (B.1.1) |
| `UpdateByID` | ✅ Default | SELECT+UPDATE qua scope, 0 rows → `ErrNotFound` |
| `DeleteByID` | ✅ Default | Same |
| `UpdateWithVersion` | ✅ Default | Trả `ErrVersionConflict` (→ 409) hoặc `ErrNotFound` (→ 404) |

`Options{SkipTenant: true}` bypass scope — **chỉ dùng cho cross-tenant admin op**, phải có comment lý do tại call site. Mặc định pass `nil` cho an toàn.

## Tại sao `Create` không auto-fill `company_id`?

Theo CLAUDE.md B.1.1: `companyActive_id` của user chỉ là scope hiển thị, không phải company nơi record thuộc về. User có quyền nhiều company → FE quyết định. Auto-fill từ JWT đã từng gây bug (`payment_crud.go::CreatePaymentClinic`).

Pattern đúng:
```go
input.CompanyID // FE gửi explicit
if !companyAllowed(c, input.CompanyID) {
    response.Validate(c, "company_id không hợp lệ")
    return
}
h.LeadRepo.Create(c, &item)
```

## Integration tests — `base_integration_test.go`

Suite gồm 12 test cover các IDOR + happy-path scenarios. Run khi có Docker:

```bash
go test -tags=integration -v ./pkg/repository/...
```

| Test | Mục đích |
|---|---|
| `FindByID_InScope_OK` | Happy path |
| `FindByID_OutOfScope_ReturnsNotFound` | **CRITICAL** — cross-tenant không leak existence |
| `FindByID_NullBranch_Accessible` | `branch_id IS NULL` records vẫn truy cập (vd emr_department toàn company) |
| `FindByID_SkipTenant_BypassesScope` | Escape hatch hoạt động |
| `List_ScopedToTenant` | List filter theo scope |
| `Count_ScopedToTenant` | Count filter theo scope |
| `UpdateByID_InScope_OK` | Happy path |
| `UpdateByID_OutOfScope_NoWrite` | **CRITICAL** — không ghi DB cross-tenant |
| `DeleteByID_InScope_OK` | Happy path |
| `DeleteByID_OutOfScope_NoDelete` | **CRITICAL** — không xoá cross-tenant |
| `UpdateWithVersion_Match_BumpsVersion` | Version bump |
| `UpdateWithVersion_Mismatch_ReturnsConflict` | 409 path |
| `UpdateWithVersion_OutOfScope_ReturnsNotFound` | **CRITICAL** — không leak qua 409 |
| `Create_NoAutoFillCompanyID` | Caller-supplied company_id được tôn trọng (B.1.1) |

Test dùng synthetic table `canary_repo` (id, label, company_id, branch_id, version) tạo trong `TestMain` qua `pkg/tenant.RegisterTestTable` (build tag `integration` only — không pollute production binary).

## Phase 2 → 3 (chưa làm)

- [ ] Run tests trên môi trường Docker để xác nhận 14 case xanh.
- [ ] Benchmark FindByID overhead vs raw GORM (chấp nhận <5%).
- [ ] 1 pilot module nhỏ migrate, manual smoke test cả 4 thao tác.

Khi 3 hạng mục trên xanh, có thể nâng status từ "Phase 2" → "Production" và bỏ warning ở đầu file.

## Tham chiếu

- Plan: `plan/09-generic-base-dao.md`
- Tenant scope: `pkg/tenant/scope.go`
- Optimistic lock: CLAUDE.md B.3
- Tenant auth matrix: CLAUDE.md B.1.1
