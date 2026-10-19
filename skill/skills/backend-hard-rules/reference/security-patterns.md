# Backend — security patterns (CRITICAL)

## Mục lục

  - Tenant scope
  - Field lock / permission — FE guard không đủ
  - Transaction + defer
  - Mass assignment
  - Optimistic lock → HTTP 409
  - Ép kiểu im lặng ở `filter[<cột>_id]`

---


Tách từ `skill/skills/backend-hard-rules/SKILL.md` (Step 4 HOW_TO_UPDATE — file gốc vượt 350 dòng). Đọc cùng `skill/skills/backend-hard-rules/SKILL.md`.

### Tenant scope

> D15/E6 (1 thương hiệu/DB, story-62): schema đã bỏ hẳn cột `company_id` — trục scope DUY NHẤT còn
> lại là `branch_id`. `ScopedDBCompanyOnly`/`GetCompanyActiveID`/`GetCompanyIDs`/`IsCompanyAllowed`/
> `JSONCompanyScope`/`RawCompanyScope` (bản company-level cũ) đã xoá (story-41). Câu nào bên dưới còn
> nhắc company mà KHÔNG đánh dấu "(đã bỏ, D15)" thì coi là lỗi chưa dọn, báo lại.

- **Default cho ownership check**: `tenant.ScopedDBNullBranch(c, db.DB, "table")` — dùng khi record là chủ thể chính của request (GET/PUT/DELETE /:id)
- **FK lookup → `tenant.CrossBranchLookup` (CRITICAL)**: khi load record liên quan qua FK (`WHERE related_id = ?` hoặc `WHERE fk IN ?`), dùng `CrossBranchLookup`/`CrossBranchLookupScope`, KHÔNG `ScopedDBNullBranch`. Branch scope không hợp lệ cho FK lookup vì record liên quan có thể thuộc branch khác (vd customer đăng ký ở chi nhánh A, đang xử lý appointment ở chi nhánh B). Ví dụ: load customer khi đang xử lý appointment → `CrossBranchLookup(h.DB, "customer")`.
  ```go
  // ❌ Sai — branch filter sẽ cắt customer thuộc branch khác → null
  tenant.ScopedDBNullBranch(c, h.DB, "customer").Where("id = ?", appt.CustomerID)

  // ✅ Đúng — FK lookup không filter branch (cố ý, không phải quên scope)
  tenant.CrossBranchLookup(h.DB, "customer").Where("id = ?", appt.CustomerID)
  ```
  Áp dụng cho: tất cả các `AttachRelation*` (mặc định dùng `CrossBranchLookupScope`), batch load bằng `id IN ?`, load child records qua `parent_id = ?` / `fk IN ?`.
  - `CrossBranchLookup` là hàm identity (trả nguyên `db`, không filter gì) — tồn tại để GIỮ DẤU Ý ĐỒ: reviewer/grep phân biệt "cố ý không lọc branch" với "quên scope", KHÔNG dùng cho list query tự do (search/paginate) hay GET/UPDATE/DELETE theo id.
- `tenant.ScopedDB` — **DEPRECATED** (dùng `ScopedDBNullBranch` thay thế)
- **Bảng mới PHẢI đăng ký `branchTables` trong `pkg/tenant/scope.go` (CRITICAL)**: `ScopedDBNullBranch` tra registry này để biết thêm WHERE `branch_id`. Bảng KHÔNG có trong map → helper trả DB **không lọc gì** (no-op zero-filter) — mọi GET/PUT/DELETE by id thành **IDOR im lặng** (build pass, test 1-tenant pass, chỉ lộ khi có tenant thứ 2). Thêm bảng có `branch_id` sở hữu dòng dữ liệu → đăng ký NGAY, verify `grep "\"<table>\"" pkg/tenant/scope.go` ra 1 hit. Chiều ngược lại cũng nguy hiểm: đăng ký `branchTables` cho bảng CHƯA migrate cột `branch_id` → không no-op mà SQL 1054 "Unknown column" crash cứng mọi query — verify cột tồn tại thật (`SHOW COLUMNS`) trước khi đăng ký, không suy từ tên bảng. Có `branch_id` KHÔNG tự động nghĩa là phải đăng ký — chỉ khi `branch_id` là QUYỀN SỞ HỮU của dòng dữ liệu, không phải tham số (xem ví dụ `warehouse`/`gcal_subscription` ở đầu `pkg/tenant/scope.go`).
  - ⚠ **(đã bỏ, D15)** Trước đây còn `tenantTables` (registry company) + các bảng dùng `companyId` JSON array (`material`, `branch`, `customer_category`...) + helper `JSONCompanyScope`/`JSONCompanyIDsAllowed`. Toàn bộ đã xoá cùng cột `company_id` (story-62) — bảng nào còn cần scope kiểu JSON multi-value cho `branch_id` thì tự thiết kế lại, chưa có helper `JSONBranchScope` tương đương.
- **Mã định danh từ hệ thống ngoài KHÔNG tự nhiên unique/scope theo tenant (CRITICAL)**: bảng định danh record bằng code từ nguồn ngoài (mã máy chấm công, mã nhập tay/import...) mà không unique constraint ở DB → PHẢI có cột `branch_id` riêng để query, KHÔNG dùng code làm khoá scope tenant. Đã cắn `hr_attendance` (định danh qua `employee_code` = `user_profile.code`, free-text không unique) trước D15 ở trục company — cùng rủi ro áp dụng cho trục `branch_id` hiện tại: bảng định danh bằng code ngoài PHẢI có `branch_id` riêng, scope qua `ScopedDBNullBranch`/`CrossBranchLookup` tuỳ ngữ cảnh, KHÔNG dùng code làm khoá. Sau khi thêm cột scope, PHẢI `grep -rn "<field_name>"` toàn repo tìm hết call site — sót 1 nơi (ZMA, job nền aggregate...) vẫn cross-tenant dù cột đã có.
- **Core ERP — NULL luôn lấy (CRITICAL)**: record `branch_id IS NULL` LUÔN được include, kể cả khi đang active branch:
  - active > 0 → `branch_id = ? OR branch_id IS NULL`
  - không active → `branch_id IN ?(user ids) OR branch_id IS NULL`
- **NULL-luôn-lấy chỉ áp dụng core-config chia sẻ chủ đích — KHÔNG áp dụng resource tenant-owned dùng chung helper NULL-fallback** (`user`, `partners`): row `branch_id IS NULL` ở các bảng này là orphan/ambiguous, không phải config dùng chung. Write nhạy cảm (reset-password/update/delete/lock/regenerate-secret) trên record NULL-branch PHẢI chặn, chỉ super-user thao tác được (`helpers.IsSuperUser`); read (list/get) vẫn giữ NULL-fallback như cũ.
- **Raw SQL scope fragment**: BẮT BUỘC bọc ngoặc `AND (x.branch_id = ? OR x.branch_id IS NULL)` — thiếu ngoặc là OR nuốt toàn bộ WHERE
- **Cache key tenant**: dùng `middleware.GetBranchActiveID` (từ user_profile, đổi khi switch branch). Sau switch branch phải gọi `middleware.InvalidateUserScope(h.Cache, userID)`
- **`branch_id` khi CREATE record con có FK tới record cha — thứ tự ưu tiên: record cha > payload (FE) > `GetBranchActiveID` (CRITICAL)**: `GetBranchActiveID` (session) có thể trả 0 hoặc khác branch của record cha khi user thao tác trên branch không phải branch đang active (multi-branch account, workspace context chưa switch). Dùng nó làm nguồn duy nhất → 2 lỗi có thể xảy ra: (1) reject nhầm dù request hợp lệ (đủ FK cha), hoặc (2) ghi branch_id lệch với branch thật của record cha.
  - **Có FK tới record cha** (`medical_record_id`, `appointment_id`, `order_service_used_id`...): load record cha qua `tenant.ScopedDBNullBranch` (verify luôn quyền truy cập, tránh cross-tenant) → lấy `branch_id` của nó làm nguồn đầu tiên. Không tìm thấy/không thuộc scope → `response.NotFound(c)`, KHÔNG fallback tiếp xuống session.
  - **Không có FK cha rõ ràng** (record độc lập — customer mới, attachment upload): FE truyền `branch_id` tường minh trong payload, BE nhận trực tiếp:
    ```go
    // ✅ Đúng — FE truyền lên, BE nhận từ form
    branchID, _ := strconv.ParseUint(c.PostForm("branch_id"), 10, 64)
    att.BranchID = helpers.UintPtr(uint(branchID))
    ```
  - Không có record cha lẫn payload → KHÔNG được fallback `GetBranchActiveID(c)` để SET giá trị persist (kể cả "chỉ set branch của chính user") — bắt buộc thêm field vào payload, FE tự tính (từ context có sẵn, vd `branchActiveId()`) rồi gửi tường minh; thiếu field → BE reject. `GetBranchActiveID` chỉ dùng để filter/guard READ, không dùng để derive giá trị WRITE (KHÔNG tự ghi NULL, KHÔNG fallback JWT — xem rule kế tiếp).

- **KHÔNG fallback tenant từ JWT khi active = 0 (CRITICAL)**: `GetCompanyID` ĐÃ XÓA khỏi middleware (2026-07-04) — JWT chỉ chứa `uid`, không có branch_id/username; KHÔNG tái tạo helper đọc branch/username từ JWT/claims. Rule này KHÔNG cấm lấy branch_id từ record cha hay payload (rule trên) — 2 nguồn đó chính xác hơn session.
  ```go
  // ✅ Đúng khi KHÔNG có record cha/payload để lấy branch_id — active = 0 → trả lỗi hoặc bỏ qua tuỳ endpoint
  bid := middleware.GetBranchActiveID(c)
  if bid == 0 {
      response.Validate(c, i18n.T(c, "branch_required"))
      return
  }
  ```
  Lý do: user có thể login rồi active ở branch khác branch lúc login (multi-branch account) — mọi nguồn "branch lúc login" đều scope sai tenant.
- Áp dụng cho GET/UPDATE/DELETE/:id — KHÔNG dùng `db.DB.First(&item, id)` trực tiếp
- **Load raw rồi verify scope sau là SAI (CRITICAL)**: pattern 2 bước (load raw → verify parent scope) bị bypass khi parent `branch_id IS NULL` (core record) — `ScopedDBNullBranch` trả count > 0 và guard pass nhầm. Load ngay qua scoped DB:
  ```go
  // ❌ Sai — load raw rồi verify sau
  h.DB.First(&visitLog, id)                               // lấy được bất kỳ tenant
  tenant.ScopedDBNullBranch(..., "emr_medical_record").   // pass nhầm nếu record IS NULL
      Where("id = ?", visitLog.MedicalRecordId).Count(&n)

  // ✅ Đúng — scoped ngay từ đầu
  tenant.ScopedDBNullBranch(c, h.DB, "emr_visit_log").First(&visitLog, id)
  ```
  Quy tắc đi kèm: mọi UPDATE/DELETE tiếp theo cũng phải qua `ScopedDBNullBranch` — `WHERE id = ?` đơn lẻ không đủ.
- **UPDATE: lock lại field tenant-anchor sau khi rebind JSON (CRITICAL)**: load record qua scoped query rồi `ShouldBindJSON(&item)` cho phép payload ghi đè `branch_id` (hoặc field unique khác dùng làm tenant anchor) → move record sang branch khác. Lưu giá trị đã load TRƯỚC khi bind, gán đè lại NGAY SAU khi bind:
  ```go
  lockedBranchID := item.BranchId
  c.ShouldBindJSON(&item)
  item.BranchId = lockedBranchID
  ```
- **`json:"-"` KHÔNG chặn được `ShouldBind` (CRITICAL)**: `ShouldBind` chọn binder theo `Content-Type` request (client tự set) — fallback form/multipart-binding đọc theo TÊN FIELD GO khi thiếu tag `form`, bỏ qua hoàn toàn tag `json:"-"`. Field nội bộ (không cho FE set) qua handler dùng `ShouldBind` phải RESET GIÁ TRỊ TƯỜNG MINH ngay sau bind (`input.X = false`), không dựa vào json tag để bảo vệ.
- **Service không có `*gin.Context`** → truyền `scopedDB *gorm.DB` từ handler; service dùng `scopedDB.Transaction` để `tx` kế thừa tenant WHERE. KHÔNG để service tự `s.DB.First(&x, id)` raw.
  ```go
  // Handler
  scopedDB := tenant.ScopedDBNullBranch(c, h.DB, "table")
  h.Svc.Update(id, input, userID, scopedDB)

  // Service
  func (s *Svc) Update(id uint, ..., scopedDB *gorm.DB) error {
      return scopedDB.Transaction(func(tx *gorm.DB) error {
          tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&old, id) // scoped + locked
      })
  }
  ```
- **Alias mismatch**: khi dùng `Table("X a")` — pass alias qua param thứ 4:
  ```go
  tenant.ScopedDBNullBranch(c, db.DB, "appointment", "a").Table("appointment a").
      Where("a.customer_id = ?", cid)
  ```
- **DELETE pattern**:
  ```go
  result := tenant.ScopedDBNullBranch(c, db.DB, "X").Table("X").Where("id = ?", id).Delete(nil)
  if result.RowsAffected == 0 { response.NotFound(c); return }
  ```
- **Đổi status trên bảng SimpleModel (KHÔNG có cột `version`): KHÔNG `load → Save` theo PK** — race: 2 request đọc status cũ rồi ghi đè nhau, mất side-effect (vd Reject đè Approve đã trừ số dư → số "used" ma). Dùng conditional update `Model(&E{}).Where("id=? AND status=?", id, expected).Updates(...)` + `RowsAffected==0 → 422 invalid_transition`. Bảng CÓ `version` thì dùng optimistic-lock 409 như thường. Đã áp: `hr_leave/handler/action.go` (Reject/Return/Cancel/Submit/Update).
- **Bảng không có `branch_id`** (tooth_bio, customer_warranty_code, check_up_general): verify qua `customerInTenant(c, customerID)` — CHỈ khi customerID đến từ input chưa xác thực (query param/filter). Nếu customerID đã verify tenant 1 lần ở đầu handler cha (vd aggregator đọc nhiều bảng con cho 1 customer detail đã xác thực) → filter thẳng `customer_id = ?`, không cần verify lặp lại.

### Field lock / permission — FE guard không đủ
- **Dữ liệu riêng tư theo QUAN HỆ thì KHÔNG được gate bằng permission (CRITICAL)**: dữ liệu chỉ một nhóm người cụ thể được đọc (kênh chat riêng, ghi chú cá nhân) phải kiểm bằng **bảng quan hệ thành viên của chính người gọi**, không phải bằng quyền. `define.IsSuperRole` (`admin`/`develop`) cho đi qua **MỌI** middleware quyền (`middleware/permission.go`), và middleware đó còn đang audit-mode ⇒ gate bằng permission fail **âm thầm**: build xanh, test permission xanh, admin vẫn đọc được dữ liệu riêng tư của người khác. Cửa kiểm quan hệ **không được có nhánh bypass** theo role/`canViewAll`/cờ cấu hình. Precedent: teamchat (module đã bỏ khỏi CRM standalone) — cấm cả việc *tạo ra* permission dạng `viewAllTeamchat`.
- **FE disable field KHÔNG đủ để khoá field (CRITICAL)**: business rule "field X không được sửa" (checkbox config, role-based lock...) chỉ disable ở FE là không đủ — client gọi thẳng API bypass được. BE phải tự `delete()` field đó khỏi payload trước update, dựa trên cùng nguồn config FE dùng.
- **Config module (`internal/modules/webapp/config/`) — ĐÚNG 1 quyền `manageConfig`, nhưng enforcer vẫn audit-mode (2026-09-02, epic `config-single-permission`)**: MỌI entry `/api/config` trong `route_perm_map.json` dùng `manageConfig` trừ ĐÚNG 1 ngoại lệ: `POST /api/config/user-profile/switch-profile` (`kind=public` — gate vào là lockout staff đa công ty). Con số tuyệt đối trôi theo mỗi route mới nên KHÔNG chép cứng — đếm lại khi cần (2026-09-07: 610/611; lúc viết 2026-09-02: 606/607):
  ```bash
  python3 -c "import json;r=json.load(open('backend/internal/modules/webapp/auth/rbac/seed/route_perm_map.json'));\
  c=[x for x in r if x.get('path','').startswith('/api/config')];\
  print(len([x for x in c if x.get('perm')=='manageConfig']),'/',len(c));\
  print([x['path'] for x in c if x.get('perm')!='manageConfig'])"
  ```
  69 key cũ (7 họ `Config*`, `Supplier`, `Warehouse`, `CashFlowCategory`, `configEInvoice`, `manageVitalType`, `manageMedRoute`, `RecordTemplate`, `ClsTemplate`) đã xoá khỏi catalog; matrix còn 1 module 1 checkbox. Quyền KHÔNG gộp (phục vụ cả ngoài màn Cấu hình): `configDevelop` (UI dev — CRM đã bỏ 2026-10-19: không route BE nào gác, FE chuyển sang `manageConfig`), `manageTeamchatOrgChannel`, `manageApprovalTemplate`, `omnichatManageAssignment`, `extOmniConversation`, `*PsychQuiz`. `.env` `PERM_ENFORCE=false` ⇒ enforcer chỉ log would-deny, nên chặn thật hôm nay đến từ **guard FE** + 5 guard tường minh trong `config/service/handler` (`define.PermManageConfig`). Siết = bật cờ global (rà teamchat + route self-service trước), KHÔNG phải thêm middleware từng route.
- **`*hasPerm` là permission-based, không phải ownership-based**: reuse 1 endpoint CRM (permission chung theo company/branch) cho trang self-service (workspace) → phải tự thêm check ownership ở BE (`created_by = userID OR JSON_CONTAINS(relatedUser, userID)`), không dựa permission đơn thuần. Precedent: `crm/reminder/handler/upsert.go` `UpdateReminder`.
- **`route_perm_map.json` là artifact TAY, KHÔNG tự sinh từ router.go**: thêm route mutating mới (POST/PUT/PATCH/DELETE trong group `authed`) phải tự thêm entry vào `internal/modules/webapp/auth/rbac/seed/route_perm_map.json`, không có script generate. Check: `go test ./internal/router/... -run TestRoutePermMapCoverage -v` — route thiếu hiện trong log "MISSING" (không fail cứng, nhưng phải tự soát). GET route mới thuộc nhóm report/list có permission gate (vd thêm 1 báo cáo công nợ sibling) → cũng nên thêm entry để đồng bộ RBAC catalog, dù `TestRoutePermMapCoverage` không tự phát hiện thiếu (test chỉ check route mutating).
- **`route_perm_map.json` KHÔNG tạo permission (CRITICAL)**: nó chỉ map route→perm. Quyền chỉ sinh ra qua `frontend/src/app/define/permission.define.ts` (`PERMISSION_SELECT`) + `permission-matrix.define.ts` (`PERMISSION_MATRIX`, tách 2026-08-28 để không vào bundle eager — sửa 1 trong 2 đều phải regen) → `node scripts/perm-codegen.mjs` → `permissions.gen.json` (2 target, `go:embed`) → seeder lúc boot upsert `rbac_auth_item`. Thiếu bước khai FE ⇒ **KHÔNG ai có quyền, kể cả super role** (`GetUserPermissions` với super = `SELECT name FROM rbac_auth_item WHERE type=2`). Gate: `TestRoutePermMapPermsInCatalog` (package `seed`) fail cứng. Đổi artifact xong phải **restart BE** mới seed. **Bất đối xứng 2 file define**: `PERMISSION_SELECT` là nguồn **sinh row** catalog; `PERMISSION_MATRIX` chỉ cấp `description` + checkbox trong màn Phân quyền ⇒ xoá khỏi matrix mà còn trong SELECT = quyền vẫn vào catalog nhưng **không ô nào cấp được** (quyền chết). **Trước khi XOÁ 1 key**: grep **chuỗi literal** của key trong `.go`, KHÔNG chỉ `grep requirePerm` — guard viết tay (`GetUserPermissions` + so chuỗi, vd `config/service/handler/guard.go`) không lộ ra qua `requirePerm`, và key đã xoá khỏi catalog thì **super role CŨNG bị chặn** ở guard đó (super = `SELECT name FROM rbac_auth_item WHERE type=2`).
- **Gom/xoá một HỌ permission: dựng danh sách key từ `route_perm_map.json` là THIẾU**: key gate 0 route không xuất hiện trong file đó, nên phải đối chiếu thêm `PERMISSION_SELECT` + `PERMISSION_MATRIX` theo tên họ. Bỏ sót ⇒ key còn row `rbac_auth_item` mà không còn checkbox = quyền chết (epic `config-single-permission`: danh sách suy-từ-route ra 50 key, thực tế phải xoá 69 — thiếu 5 `ConfigZaloTimeFrame` + 4 `ext*`). Sweep FE phải rà **hai chiều**: key bị xoá (`tsc` bắt được) VÀ key còn SỐNG nhưng gate sai màn (`tsc` **KHÔNG** bắt — biểu hiện chỉ là nút CUD biến mất / form khoá chỉ-đọc; đã cắn 9 chỗ `*PsychQuiz` gate màn cấu hình sau khi BE đã đổi sang `manageConfig`).
- **`perm` trong `route_perm_map.json` có thể là danh sách phẩy = OR** (`"perm": "a,b"`): script kiểm/sửa file này phải tách `,` trước khi so với catalog — không tách ra dương tính giả "perm không có trong catalog". Tin `TestRoutePermMapPermsInCatalog` hơn script tự viết.
- **Self-service route KHÔNG gate perm module/admin trong `route_perm_map.json` (CRITICAL)**: endpoint thao tác trên chính `GetUserID(c)` + verify ownership trong handler (đổi mật khẩu, switch company/branch active...) PHẢI để `kind=public`/whitelist, `enforce_phase1=false`. Gate perm coarse (vd `updateUser`, `extConfigSystem`) → user thường thiếu quyền bị 403 khi `PERM_ENFORCE=true` = regression tính năng self-service (lockout). Đã cắn: `update-password` (updateUser), `auth/scope` + `config/user-profile/switch-profile` (extConfigSystem → chặn company picker của staff đa-cty).
- **Route nhạy cảm cần LUÔN enforce bất kể rollout `PERM_ENFORCE` (CRITICAL)**: `enforce_phase1:true` trong `route_perm_map.json` KHÔNG nghĩa là route đó luôn bị chặn — `PermissionEnforcer` dùng chung 1 flag global `PERM_ENFORCE` cho MỌI route (`decide()`). Route sửa dữ liệu tài chính đã phát sinh giao dịch → tự thêm guard tường minh trong handler (`rbacSvc.IsSuperUser`/`GetUserPermissions` + check thủ công), KHÔNG chỉ dựa route_perm_map.
- **Trang Detail (CustomerDetail/BrokerDetail/EMR examination...) KHÔNG gate permission theo tab, API GET phục vụ tab cũng không check permission (known gap, user nêu 2026-08-04)**: vào được trang detail (qua 1 permission list/menu duy nhất) là thấy hết mọi tab + data — tab con (lịch sử điều trị, thanh toán, EMR...) không tự check quyền riêng dù nghiệp vụ có thể coi đó là quyền khác nhau (vd `viewCustomerPayment` khác `viewCustomer`); BE GET route phục vụ các tab đó cũng không có `requirePerm` riêng, chỉ chung `authed` (JWTAuth). Verify cụ thể ở `emr/examination/:id` (`ExaminationPage`): route chỉ gate `permissionViewReception`, 8 tab bên trong (exam/orders/conclusion/linked/payment/nursing/followup/history) không tab nào check quyền riêng — comment trong code còn ghi thẳng "Tab Chăm sóc ĐD / Theo dõi điều trị hiển thị cho mọi user (không gate quyền)" (`examination.ts`). Rõ nhất là tab **Payment**: `emr-payment-list.ts` chỉ check permission ở action Create/Update/Delete, KHÔNG check `permissionViewPayment` khi list/xem — trong khi route riêng `/emr/billing` (route độc lập) lại gate `permissionViewPayment` đàng hoàng → user chỉ có `permissionViewReception` vẫn xem được hết lịch sử thanh toán qua tab. BE: GET list payment không có entry trong `route_perm_map.json` (chỉ track route mutating). Không áp dụng cho module `workspace` — đó là app riêng (self-service theo `GetUserID(c)`), không phải trang detail admin xem người khác. Root cause giống gap `config/` module ở trên: FE `*hasPerm` (nếu có) chỉ ẩn UI, chưa có middleware/route_perm_map cho các GET route theo tab. Chưa fix — biết trước khi review để không báo trùng lặp; nếu cần siết, phải map từng tab detail → permission riêng rồi thêm `requirePerm` cho cả route GET và ẩn tab FE tương ứng, không chỉ ẩn tab. **Ngoại lệ — `cls-result` + `patient-document` là CHỦ ĐÍCH không gate** (commit FE `99f2c60ba` 2026-08-04: "mọi role vào được phiếu khám đều xem/thêm/sửa/xoá"), 14 entry route_perm_map để `kind=public`/`perm:""`; KHÔNG khai lại `*ClsResult`/`*PatientDocument` vào `permission.define.ts`. Bảo vệ còn lại: `requirePerm(viewReception)` ở route cha + `@if(!isLocked)`.
- **`CustomerDetail` — CHỦ ĐÍCH chỉ gate `viewCustomer`, KHÔNG gate theo tab (user chốt 2026-08-14)**: `/crm/customer/detail/:id` gate DUY NHẤT bằng `requirePerm(permissionViewCustomer)` ở route cha `customer` (`frontend/src/app/modules/crm/crm.routes.ts:27` — route `detail/:id` trong `customer.routes.ts` không có guard riêng, thừa hưởng cha). Vào được trang = xem được TOÀN BỘ 13 tab + data. **KHÔNG thêm `requirePerm`/`*hasPerm` view cho từng tab, KHÔNG thêm permission check vào các GET API con phục vụ tab** (payment/order-plan/treatment-course/labo/warranty/care/prescription/check-up/psych/reminder/appointment) — đây là quyết định nghiệp vụ, không phải nợ kỹ thuật. Verify 2026-08-14: 13 component tab, không component nào gate view (`payment-of-customer`, `order-plan-in-customer`, `treatment-course-list-in-customer`... chỉ có `*hasPerm` cho Create/Update/Delete). **2 ngoại lệ CỐ Ý giữ lại** (user xác nhận, không được gỡ khi "dọn cho đồng nhất"): (1) tab 7 "Hình ảnh" gate `permissionViewImage` ở CẢ `*hasPerm` trong `customer-detail.html` LẪN `visibleTabValues()` (`customer-detail.ts:284`) — phải sửa cả 2 chỗ nếu đổi, chỉ sửa template thì vào bằng `?tab=7` vẫn lọt; (2) nút "xem chi tiết" phiên trắc nghiệm trong tab 13 gate `permissionViewPsychQuiz` (`psych-session-of-customer.html:109`) — đây là gate ĐIỀU HƯỚNG sang trang khác, không phải gate list. Quyền CUD trong tab giữ nguyên toàn bộ: `viewCustomer` chỉ quy định quyền XEM. Nút "Sửa" theo `status` (`preparePermissions`, `customer-detail.ts:735`): lead-like → `updateLead`; đã chuyển KH → `updateCustomer`; còn lại → một trong hai. Tab 3/9 (`isDental`), 12 (`isCheckUpGeneralVisible`), 13 (`isPsychiatryVisible`) ẩn/hiện theo **chuyên khoa**, KHÔNG phải permission — đừng nhầm thành gate quyền. ⚠ Review/QA: KHÔNG báo "tab X thiếu permission view" ở `CustomerDetail` nữa — đã đóng bằng quyết định, không phải bỏ sót. Các trang detail khác (`BrokerDetail`, `emr/examination/:id`...) vẫn ở trạng thái gap mô tả ở gạch đầu dòng trên, chưa có quyết định tương tự.
- **RBAC role mutation (create/update/delete role, gán quyền cho role) chỉ cần `requirePerm(createRole/updateRole/deleteRole)`** — KHÔNG còn bắt buộc `requireSuper()` (đổi policy 2026-07-14, user được phân công quyền này tự sửa được). Role `"admin"` (`define.RoleAdmin`) LUÔN bị chặn xóa cứng trong `DeleteRoleName` bất kể permission. **Permission catalog (type=2, global không gắn 1 role — `CreatePermissionName`/`Update.../Delete...`) vẫn bắt buộc `requireSuper()`** — không có permission per-role nào hợp lý để gate loại mutation này.

### Transaction + defer
- **LUÔN re-panic sau rollback** trong defer:
  ```go
  // ❌ Sai — nuốt panic, Gin recovery không nhận, request treo
  defer func() {
      if r := recover(); r != nil { tx.Rollback() }
  }()

  // ✅ Đúng
  defer func() {
      if r := recover(); r != nil { tx.Rollback(); panic(r) }
  }()
  ```
  Xem WalkIn (`reception/handler.go`) và DeleteMedicalRecord (`medical_record/handler/handler.go`) làm chuẩn.
- **Check `.Error` NGAY sau MỌI statement** (`tx.Exec`, `tx.Model().Updates`...) bên trong transaction thủ công, không chỉ `tx.Commit()` — thiếu check khiến 1 lệnh fail âm thầm nhưng Commit vẫn "thành công", trả response success giả dù dữ liệu ghi thiếu/sai một phần. Rollback + return ngay khi có lỗi. Xem `RegenerateSecret` (`hr/partner/handler/handler.go`) làm chuẩn.

### Mass assignment
- KHÔNG `json.Marshal(params) + Unmarshal(&struct)` trực tiếp
- Dùng whitelist map:
  ```go
  var allowedFields = map[string]bool{"fullname": true, "phone": true, ...}
  func FilterAllowed(body map[string]interface{}) map[string]interface{} { ... }
  ```
- **Struct-bind vào record đã Load cũng dính**: `First(&item)` (đã scope-verify quyền sở hữu) rồi `ShouldBindJSON(&item)` — field ownership/FK (vd `customer_id`) bị request ghi đè tự do, vô hiệu hoá check quyền vừa làm. Khoá lại field đó về giá trị gốc NGAY SAU bind, trước validate/Save.
- ⚠ **Mặt trái — whitelist là dao 2 lưỡi**: field FE gửi mà THIẾU trong map bị drop **im lặng**, BE vẫn trả 200. User nhập xong bấm Lưu, dữ liệu biến mất không dấu vết, không log. Ca thật: form bệnh nhân EMR khai 60 key / bảng `customer` có 37 → 23 field (cả tab "Đồng ý", dị ứng thuốc) mất trắng, tồn tại nhiều tháng.
- Thêm field vào form → phải đủ **3 chỗ**: cột thật (migration) + field entity + allowlist. Thiếu 1 trong 3 là mất im lặng.
- Verify bằng script đối chiếu key `getInitialObject()` của FE với json tag entity BE — KHÔNG tin mắt đọc.

### Optimistic lock → HTTP 409
- Service trả sentinel `var ErrVersionConflict = errors.New(...)`
- Handler: `response.HandleError(c, err)` — tự map → 409
- KHÔNG trả 400
- FE BẮT BUỘC gửi `version` trong body

## Ép kiểu im lặng ở `filter[<cột>_id]` — cửa thứ ba của lớp bug "id chưa parse ra số"

Rule + cách dùng: `../SKILL.md` §"Path param `id` PHẢI parse ra số", mục "Lớp thứ ba". Đây là số đo
và thủ tục bảo trì allowlist.

`query.ApplyFilter` từng đẩy **chuỗi thô** vào `Where(col + " = ?", v)`. Đo trên bench 545.547 dòng
`customer_warranty_card` (2026-09-08):

| `filter[customer_id]` | Số dòng trả về |
|---|---|
| `888002` | 5.000 |
| **`888002abc`** | **5.000** ← đọc TRÚNG dữ liệu khách 888002 |
| `" 888002"` (có space) | 5.000 |
| `888002.9` | 0 |

`SHOW WARNINGS` ⇒ `Warning 1292 Truncated incorrect DOUBLE value: '888002abc'` — MySQL CÓ báo, GORM
không đọc nên không ai thấy.

Hành vi sau khi vá (`pkg/query/numeric_id_filter.go`), chỉ áp cho cột trong allowlist:

| Đường | Giá trị không parse được | Kết quả |
|---|---|---|
| `eq` · `in` · `gt` · `gte` · `lt` · `lte` | `888002abc` | `1 = 0` ⇒ rỗng |
| `neq` | `888002abc` | bỏ mệnh đề (khác một id không tồn tại đúng với MỌI dòng) |
| `in` hỗn hợp | `[101, 102abc, 103]` | giữ `[101, 103]` |
| `like` · `is` · `nis` | — | không đụng |

Chọn "trả rỗng" chứ KHÔNG "bỏ qua filter": bỏ qua sẽ **nới rộng** phạm vi ra dữ liệu bản ghi khác —
sai hướng an toàn.

**Thêm cột vào allowlist** chỉ sau khi kiểm nó thuần số ở MỌI bảng — cột `*_id` kiểu `varchar` mà
lọt vào danh sách sẽ biến `filter[template_id]=abc123` thành "không khớp gì":

```sql
SELECT COLUMN_NAME, GROUP_CONCAT(DISTINCT DATA_TYPE), COUNT(DISTINCT TABLE_NAME)
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA=DATABASE() AND COLUMN_NAME='<tên>' GROUP BY COLUMN_NAME;
```

`service_type_id` CỐ Ý ngoài allowlist: `int` ở `service`, `json` ở `treatment_course_type`.

Lưới: `pkg/query/numeric_id_filter_test.go` (mutation-verify: gỡ guard ⇒ test đỏ). Gate
`verify-id-params.sh` KHÔNG soi cửa này.
