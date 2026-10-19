//go:build integration

// Integration test — `POST /api/config/onboarding/bootstrap` trên DB dùng-một-lần.
//
// ⚠ Handler dùng MySQL named lock trên 1 `*sql.Conn` riêng rồi mở tx qua pool. Suite của
// testhelpers giới hạn MỘT connection ⇒ handler chạy trên `testDB` sẽ tự khoá chết. Vì vậy handler
// nhận `bootstrapPool(t)` — pool nhiều connection mở từ cùng TEST_MYSQL_DSN (FK checks BẬT như
// production); còn seed/đọc kiểm tra vẫn qua `testDB` (FK checks tắt, seed không cần đồ thị đủ).
package onboarding

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"erpCluvix/internal/middleware"
	auditDao "erpCluvix/internal/modules/webapp/auth/audit/dao"
	branchHandler "erpCluvix/internal/modules/webapp/config/branch/handler"
	"erpCluvix/internal/modules/webapp/config/onboarding/handler"
	"erpCluvix/internal/modules/webapp/config/onboarding/service"
	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/brand"
	"erpCluvix/pkg/cache"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/dexie"
	"erpCluvix/pkg/i18n"
	"erpCluvix/pkg/seclog"
)

const bootstrapPath = "/api/config/onboarding/bootstrap"

var (
	poolOnce sync.Once
	pool     *gorm.DB
	poolErr  error
)

// bootstrapPool — pool nhiều connection cho handler (xem comment đầu file). Khởi tạo 1 lần; kèm
// seclog sink ghi thật vào `security_audit_log` (khuôn bootstrap/app.go) để AC9 query được bảng.
func bootstrapPool(t *testing.T) *gorm.DB {
	t.Helper()
	poolOnce.Do(func() {
		pool, poolErr = gorm.Open(mysql.Open(os.Getenv("TEST_MYSQL_DSN")), &gorm.Config{Logger: gormlogger.Discard})
		if poolErr != nil {
			return
		}
		sqlDB, err := pool.DB()
		if err != nil {
			poolErr = err
			return
		}
		sqlDB.SetMaxOpenConns(20)
		seclog.InitSink(256, (&auditDao.Dao{DB: pool}).InsertBatch)
	})
	if poolErr != nil {
		t.Fatalf("mở pool test: %v", poolErr)
	}
	return pool
}

// resetOnboardingData — đưa DB về "trắng" cho nhóm test bootstrap (story-62: chưa chi nhánh, chưa
// dòng `business_profile`); dọn cả khi test kết thúc để các test status (requireBranchEmpty) không
// nhận dữ liệu bẩn. Mọi lệnh dọn kiểm `.Error`. `brand.Invalidate()` — cache hồ sơ in-memory không
// được mang policy của test trước sang test sau.
func resetOnboardingData(t *testing.T) {
	t.Helper()
	wipe := func(report func(string, ...any)) {
		for _, q := range []string{"DELETE FROM config_code", "DELETE FROM branch", "DELETE FROM business_profile"} {
			if err := testDB.Exec(q).Error; err != nil {
				report("dọn dữ liệu (%s): %v", q, err)
			}
		}
		brand.Invalidate()
	}
	wipe(t.Fatalf)
	t.Cleanup(func() { wipe(t.Errorf) })
}

func validProfileJSON(name, code string) string {
	return fmt.Sprintf(`{"name":%q,"code":%q,"business_type_id":%d}`, name, code, define.BusinessClinic)
}

func validBranchJSON(name, code string) string {
	return fmt.Sprintf(`{"name":%q,"code":%q,"address":"12 Nguyễn Trãi"}`, name, code)
}

type bootstrapRouterOpts struct {
	withAuthHeaders bool // gắn AuthResponseHeaders THẬT (cần cho AC10)
	cache           *cache.RedisCache
}

// newBootstrapRouter — userID (việc của JWTAuth) → [AuthResponseHeaders thật] → group /api/config →
// RegisterRoutes của module; kèm 1 route KHÔNG miễn gate (`/api/crm/customer`) làm đối chứng.
func newBootstrapRouter(t *testing.T, userID int, o bootstrapRouterOpts) *gin.Engine {
	t.Helper()
	db := bootstrapPool(t)
	r := testhelpers.NewGin()
	r.Use(func(c *gin.Context) { c.Set("userID", uint(userID)); c.Next() })
	if o.withAuthHeaders {
		r.Use(middleware.AuthResponseHeaders(db, o.cache))
	}
	RegisterRoutes(r.Group("/api/config"), handler.Deps{DB: db, Store: nil, Cache: o.cache})
	r.GET("/api/crm/customer", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"reached": true}) })
	return r
}

func postBootstrap(t *testing.T, r *gin.Engine, fields map[string]string) (int, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := mw.WriteField(k, fields[k]); err != nil {
			t.Fatalf("multipart field %s: %v", k, err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("multipart close: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, bootstrapPath, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code, w.Body.String()
}

func getPath(r *gin.Engine, path string) (int, string) {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w.Code, w.Body.String()
}

type bootstrapResp struct {
	Data struct {
		// FECompat — key FE-compat `company_id` đã gỡ ở story-66: phải vắng (nil).
		FECompat *uint `json:"company_id"`
		BranchID *uint `json:"branch_id"`
	} `json:"data"`
	Message string `json:"message"`
}

func parseBootstrapResp(t *testing.T, body string) bootstrapResp {
	t.Helper()
	var resp bootstrapResp
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("body không phải JSON: %v — %s", err, body)
	}
	return resp
}

func msg(key string) string { return i18n.TL(i18n.Vi, key) }

// dbSnapshot — mọi thứ bootstrap có thể ghi; so trước/sau cho các ca "DB không đổi".
type dbSnapshot struct {
	Profiles, Branches, ConfigCodes int64
	ProfileName, ProfileCode        sql.NullString // dòng business_profile id nhỏ nhất (nếu có)
	UserBranchIDs                   sql.NullString
	ActiveBranch                    sql.NullInt64
}

func snapshot(t *testing.T, userID int) dbSnapshot {
	t.Helper()
	var s dbSnapshot
	for _, x := range []struct {
		table string
		dst   *int64
	}{{"business_profile", &s.Profiles}, {"branch", &s.Branches}, {"config_code", &s.ConfigCodes}} {
		if err := testDB.Table(x.table).Count(x.dst).Error; err != nil {
			t.Fatalf("đếm %s: %v", x.table, err)
		}
	}
	if err := testDB.Raw("SELECT name, code FROM business_profile ORDER BY id LIMIT 1").
		Row().Scan(&s.ProfileName, &s.ProfileCode); err != nil && !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("đọc business_profile: %v", err)
	}
	if err := testDB.Raw("SELECT branchId FROM `user` WHERE id = ?", userID).
		Row().Scan(&s.UserBranchIDs); err != nil {
		t.Fatalf("đọc user: %v", err)
	}
	// Không có profile (ca rollback AC5) ⇒ giữ NULL; lỗi khác vẫn Fatal.
	if err := testDB.Raw("SELECT branchActive_id FROM user_profile WHERE user_id = ?", userID).
		Row().Scan(&s.ActiveBranch); err != nil && !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("đọc user_profile: %v", err)
	}
	return s
}

func requireSnapshotEqual(t *testing.T, before, after dbSnapshot) {
	t.Helper()
	if before != after {
		t.Fatalf("DB bị đổi dù request bị từ chối.\n trước: %+v\n sau:   %+v", before, after)
	}
}

// bootstrapLockName — tên khoá THẬT cho DB test, dựng bằng CÙNG hàm service dùng.
func bootstrapLockName(t *testing.T) string {
	t.Helper()
	var dbName string
	if err := testDB.Raw("SELECT DATABASE()").Row().Scan(&dbName); err != nil || dbName == "" {
		t.Fatalf("đọc DATABASE(): %q %v", dbName, err)
	}
	return service.BootstrapLockName(dbName)
}

// requireLockFree — khoá bootstrap phải được NHẢ khi request trả về. Kiểm qua `testDB` (pool 1 conn
// riêng ⇒ session KHÁC pool của handler). `conn.Close()` chỉ trả conn về pool, KHÔNG đóng session ⇒
// thiếu RELEASE_LOCK thì khoá kẹt và IS_FREE_LOCK = 0.
func requireLockFree(t *testing.T) {
	t.Helper()
	name := bootstrapLockName(t)
	var free sql.NullInt64
	if err := testDB.Raw("SELECT IS_FREE_LOCK(?)", name).Row().Scan(&free); err != nil {
		t.Fatalf("IS_FREE_LOCK: %v", err)
	}
	if !free.Valid || free.Int64 != 1 {
		t.Fatalf("named lock %q chưa được nhả sau request (IS_FREE_LOCK=%v)", name, free)
	}
}

// resetDexieVersions — `config_systems` 1 dòng `{}` (schema test không seed dòng nào, mà
// BumpDexieTable chỉ UPDATE ⇒ bảng rỗng thì assert bump luôn vô nghĩa).
func resetDexieVersions(t *testing.T) {
	t.Helper()
	for _, q := range []string{"DELETE FROM config_systems", "INSERT INTO config_systems (content) VALUES ('{}')"} {
		if err := testDB.Exec(q).Error; err != nil {
			t.Fatalf("seed config_systems (%s): %v", q, err)
		}
	}
	t.Cleanup(func() {
		if err := testDB.Exec("DELETE FROM config_systems").Error; err != nil {
			t.Errorf("dọn config_systems: %v", err)
		}
	})
}

// dexieVersions — `content.dexieVersions` (map table → unix) như pkg/dexie.BumpDexieTable ghi.
func dexieVersions(t *testing.T) map[string]int64 {
	t.Helper()
	var raw sql.NullString
	if err := testDB.Raw("SELECT JSON_EXTRACT(content, ?) FROM config_systems", "$."+dexie.OptionDexieVersions).
		Row().Scan(&raw); err != nil {
		t.Fatalf("đọc config_systems: %v", err)
	}
	out := map[string]int64{}
	if raw.Valid && raw.String != "" {
		if err := json.Unmarshal([]byte(raw.String), &out); err != nil {
			t.Fatalf("dexieVersions không phải map: %v — %s", err, raw.String)
		}
	}
	return out
}

// holdLockElsewhere giữ named lock bootstrap (tên THẬT của DB test) trên 1 connection khác cho tới cleanup.
func holdLockElsewhere(t *testing.T) {
	t.Helper()
	holdNamedLock(t, bootstrapLockName(t))
}

func holdNamedLock(t *testing.T, lockName string) {
	t.Helper()
	sqlDB, err := bootstrapPool(t).DB()
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	conn, err := sqlDB.Conn(context.Background())
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	var got sql.NullInt64
	if err := conn.QueryRowContext(context.Background(), "SELECT GET_LOCK(?, 1)", lockName).Scan(&got); err != nil || got.Int64 != 1 {
		_ = conn.Close()
		t.Fatalf("không giữ được lock đối chứng: got=%v err=%v", got, err)
	}
	t.Cleanup(func() {
		_, _ = conn.ExecContext(context.Background(), "SELECT RELEASE_LOCK(?)", lockName)
		_ = conn.Close()
	})
}

// ── AC1 ─────────────────────────────────────────────────────────────────────────────────────

func TestBootstrap_NonSuper_403_NoChange(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	resetOnboardingData(t)
	seedUser(t, 911, false)
	before := snapshot(t, 911)

	code, body := postBootstrap(t, newBootstrapRouter(t, 911, bootstrapRouterOpts{}), map[string]string{
		"company": validProfileJSON("Brand AC1", "AC1"),
		"branch":  validBranchJSON("Branch AC1", "BAC1"),
	})
	if code != http.StatusForbidden {
		t.Fatalf("HTTP = %d, muốn 403 cho user KHÔNG super. Body: %s", code, body)
	}
	requireSnapshotEqual(t, before, snapshot(t, 911))
}

// Lỗi SQL ở truy vấn quyền super PHẢI là 500, không bị che thành 403 (IsSuperUserE). Gây lỗi thật
// bằng cách giấu bảng `rbac_auth_assignment`; không gắn AuthResponseHeaders (middleware cũng đọc
// bảng đó nhưng nuốt lỗi — status code sẽ không còn nói gì về handler).
func TestBootstrap_SuperQueryError_500(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	resetOnboardingData(t)
	seedUser(t, 912, true)
	r := newBootstrapRouter(t, 912, bootstrapRouterOpts{})

	if err := testDB.Exec("RENAME TABLE rbac_auth_assignment TO rbac_auth_assignment_onbwiz_hidden").Error; err != nil {
		t.Fatalf("không gây được lỗi query quyền: %v", err)
	}
	t.Cleanup(func() {
		if err := testDB.Exec("RENAME TABLE rbac_auth_assignment_onbwiz_hidden TO rbac_auth_assignment").Error; err != nil {
			t.Errorf("KHÔNG khôi phục được bảng rbac_auth_assignment — test sau sẽ hỏng: %v", err)
		}
	})

	code, body := postBootstrap(t, r, map[string]string{"company": validProfileJSON("Brand Err", "ERR")})
	if code != http.StatusInternalServerError {
		t.Fatalf("HTTP = %d, muốn 500 — lỗi SQL khi kiểm super đang bị che. Body: %s", code, body)
	}
	for _, table := range []string{"business_profile", "branch"} {
		var n int64
		if err := testDB.Table(table).Count(&n).Error; err != nil {
			t.Fatalf("đếm %s: %v", table, err)
		}
		if n != 0 {
			t.Fatalf("%s = %d, muốn 0 khi kiểm quyền lỗi", table, n)
		}
	}
}

// ── AC2 ─────────────────────────────────────────────────────────────────────────────────────

// Story-62: "đã onboarding" = đã có chi nhánh (bảng `branch` không rỗng).
func TestBootstrap_AlreadyHasBranch_422_NoChange(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	resetOnboardingData(t)
	seedUser(t, 913, true)
	if err := testDB.Exec("INSERT INTO business_profile (id, name, code) VALUES (5101, 'Existing AC2', 'EX2')").Error; err != nil {
		t.Fatalf("seed business_profile: %v", err)
	}
	if err := testDB.Exec("INSERT INTO branch (id, name, code) VALUES (5101, 'Existing AC2', 'EXB2')").Error; err != nil {
		t.Fatalf("seed branch: %v", err)
	}
	before := snapshot(t, 913)

	code, body := postBootstrap(t, newBootstrapRouter(t, 913, bootstrapRouterOpts{}), map[string]string{
		"company": validProfileJSON("Brand AC2", "AC2"),
		"branch":  validBranchJSON("Branch AC2", "BAC2"),
	})
	if code != http.StatusUnprocessableEntity || !strings.Contains(body, msg("onboarding_already_done")) {
		t.Fatalf("HTTP = %d, muốn 422 onboarding_already_done. Body: %s", code, body)
	}
	requireSnapshotEqual(t, before, snapshot(t, 913))
}

// ── AC3 + AC7 + AC9 ─────────────────────────────────────────────────────────────────────────

type configCodeRow struct {
	ParentTable       string `gorm:"column:parent_table"`
	Name              string `gorm:"column:name"`
	InfixCode         string `gorm:"column:infix_code"`
	IncludeBranchCode bool   `gorm:"column:include_branch_code"`
	MonthlyReset      bool   `gorm:"column:monthly_reset"`
	SerialLength      int    `gorm:"column:serial_length"`
}

func configCodeSet(t *testing.T, db *gorm.DB, branchID uint) []configCodeRow {
	t.Helper()
	var rows []configCodeRow
	if err := db.Table("config_code").
		Select("parent_table, name, infix_code, include_branch_code, monthly_reset, serial_length").
		Where("branch_id = ?", branchID).
		Order("parent_table, name").Scan(&rows).Error; err != nil {
		t.Fatalf("đọc config_code: %v", err)
	}
	return rows
}

var errRollbackControl = errors.New("rollback control seed")

// controlConfigCodeSet — tập mà SeedConfigCodeForBranch sinh khi gọi TRỰC TIẾP (DB đối chứng: một
// tx bị rollback, id chi nhánh giả không đụng dữ liệu thật).
func controlConfigCodeSet(t *testing.T, uid int) []configCodeRow {
	t.Helper()
	const ctrlBranch = 990001
	var rows []configCodeRow
	err := testDB.Transaction(func(tx *gorm.DB) error {
		// Story-59: row `service` là 1 row DÙNG CHUNG toàn hệ thống (Seed chỉ tạo khi CHƯA có row service
		// nào). Bootstrap vừa tạo row đó ⇒ đối chứng phải chạy trên trạng thái "chưa có service" như lúc
		// bootstrap chạy (tx này rollback, không đụng dữ liệu).
		if err := tx.Exec("DELETE FROM config_code WHERE parent_table = 'service'").Error; err != nil {
			return err
		}
		if err := branchHandler.SeedConfigCodeForBranch(tx, ctrlBranch, uid); err != nil {
			return err
		}
		rows = configCodeSet(t, tx, ctrlBranch)
		return errRollbackControl
	})
	if !errors.Is(err, errRollbackControl) {
		t.Fatalf("seed đối chứng: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("tập đối chứng rỗng — so sánh vô nghĩa")
	}
	return rows
}

// columnDefault — COLUMN_DEFAULT của information_schema (NULL ⇒ Valid=false).
func columnDefault(t *testing.T, table, column string) sql.NullString {
	t.Helper()
	var d sql.NullString
	if err := testDB.Raw("SELECT COLUMN_DEFAULT FROM information_schema.COLUMNS "+
		"WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?", table, column).
		Row().Scan(&d); err != nil {
		t.Fatalf("đọc default %s.%s: %v", table, column, err)
	}
	return d
}

// Chưa có dòng `business_profile` ⇒ bootstrap INSERT đúng 1 dòng; field ngoài allowlist (policy bảo
// mật, id, status…) bị bỏ qua — cột policy mang DEFAULT của DB.
func TestBootstrap_WithBranch_201(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	resetOnboardingData(t)
	const uid = 914
	seedUser(t, uid, true)

	// AC7: nhét thêm field ngoài allowlist — phải bị bỏ qua.
	profileJSON := fmt.Sprintf(`{"id":777777,"name":"Brand AC3","code":"AC3","business_type_id":%d,`+
		`"status":0,"allowed_ips":["1.2.3.4"],"require_2fa":1,"require_2fa_roles":["admin"],`+
		`"totp_trust_days":99,"SortOrder":424242,"hotline":"0900000000","companyColor":"#123456"}`, define.BusinessSpa)
	branchJSON := `{"id":888888,"name":"Branch AC3","code":"BAC3","address":"1 Lê Lợi"}`
	resetDexieVersions(t)

	code, body := postBootstrap(t, newBootstrapRouter(t, uid, bootstrapRouterOpts{}), map[string]string{
		"company": profileJSON,
		"branch":  branchJSON,
	})
	if code != http.StatusCreated {
		t.Fatalf("HTTP = %d, muốn 201. Body: %s", code, body)
	}
	resp := parseBootstrapResp(t, body)
	if resp.Data.FECompat != nil || resp.Data.BranchID == nil || *resp.Data.BranchID == 0 {
		t.Fatalf("data phải có branch_id và KHÔNG còn key FE-compat company_id (story-66). Body: %s", body)
	}
	bid := *resp.Data.BranchID

	var nProfile, nBranch int64
	if err := testDB.Table("business_profile").Count(&nProfile).Error; err != nil {
		t.Fatal(err)
	}
	if err := testDB.Table("branch").Count(&nBranch).Error; err != nil {
		t.Fatal(err)
	}
	if nProfile != 1 || nBranch != 1 {
		t.Fatalf("business_profile=%d branch=%d, muốn 1/1", nProfile, nBranch)
	}
	if bid == 888888 {
		t.Fatalf("id client gửi được dùng làm khoá chính branch: %d", bid)
	}
	var bp struct {
		ID              uint           `gorm:"column:id"`
		Name            string         `gorm:"column:name"`
		Code            string         `gorm:"column:code"`
		Hotline         string         `gorm:"column:hotline"`
		Timezone        string         `gorm:"column:timezone"`
		CreatedBy       sql.NullInt64  `gorm:"column:created_by"`
		Require2FA      sql.NullString `gorm:"column:require_2fa"`
		TotpTrustDays   sql.NullString `gorm:"column:totp_trust_days"`
		AllowedIPs      sql.NullString `gorm:"column:allowed_ips"`
		Require2FARoles sql.NullString `gorm:"column:require_2fa_roles"`
	}
	if err := testDB.Raw("SELECT id, name, code, hotline, timezone, created_by, " +
		"CAST(require_2fa AS CHAR) AS require_2fa, CAST(totp_trust_days AS CHAR) AS totp_trust_days, " +
		"allowed_ips, require_2fa_roles FROM business_profile").Scan(&bp).Error; err != nil {
		t.Fatalf("đọc business_profile: %v", err)
	}
	if bp.ID == 777777 {
		t.Errorf("id client gửi được dùng làm khoá chính business_profile")
	}
	if bp.Name != "Brand AC3" || bp.Code != "AC3" || bp.Hotline != "0900000000" {
		t.Errorf("field allowlist không được ghi: name=%q code=%q hotline=%q", bp.Name, bp.Code, bp.Hotline)
	}
	if bp.CreatedBy.Int64 != uid {
		t.Errorf("business_profile created_by=%v, muốn %d", bp.CreatedBy, uid)
	}
	if def := columnDefault(t, "business_profile", "timezone"); !def.Valid || bp.Timezone != def.String {
		t.Errorf("timezone = %q, muốn default DB %v (payload không gửi)", bp.Timezone, def)
	}
	for _, x := range []struct {
		col string
		got sql.NullString
	}{{"require_2fa", bp.Require2FA}, {"totp_trust_days", bp.TotpTrustDays}} {
		if def := columnDefault(t, "business_profile", x.col); x.got != def {
			t.Errorf("business_profile.%s = %v, muốn default DB %v — field ngoài allowlist lọt vào", x.col, x.got, def)
		}
	}
	if bp.AllowedIPs.Valid || bp.Require2FARoles.Valid {
		t.Errorf("allowed_ips=%v require_2fa_roles=%v, muốn NULL — field ngoài allowlist lọt vào", bp.AllowedIPs, bp.Require2FARoles)
	}

	// branch
	var br struct {
		SortOrder sql.NullInt64 `gorm:"column:SortOrder"`
	}
	if err := testDB.Raw("SELECT SortOrder FROM branch WHERE id = ?", bid).Scan(&br).Error; err != nil {
		t.Fatalf("đọc branch: %v", err)
	}
	if br.SortOrder.Int64 != int64(bid) {
		t.Errorf("branch SortOrder = %v, muốn %d", br.SortOrder, bid)
	}

	// config_code == đúng tập SeedConfigCodeForBranch sinh
	got := configCodeSet(t, testDB, bid)
	want := controlConfigCodeSet(t, uid)
	if fmt.Sprintf("%+v", got) != fmt.Sprintf("%+v", want) {
		t.Errorf("config_code của branch khác tập đối chứng.\n got:  %+v\n want: %+v", got, want)
	}

	// user + profile
	s := snapshot(t, uid)
	if !jsonArrayHas(s.UserBranchIDs.String, bid) {
		t.Errorf("user.branchId=%v, muốn chứa %d", s.UserBranchIDs, bid)
	}
	if s.ActiveBranch.Int64 != int64(bid) {
		t.Errorf("profile branchActive_id = %v, muốn %d", s.ActiveBranch, bid)
	}

	// AC9: security_audit_log (sink flush theo ticker 2s ⇒ chờ)
	requireAuditRow(t, uid)

	// Khoá đã nhả sau ca thành công.
	requireLockFree(t)

	// Dexie bump (Hard Rule sau CUD): branches.
	if v := dexieVersions(t); v["branches"] <= 0 {
		t.Errorf("dexieVersions = %v, muốn có branches sau bootstrap", v)
	}
}

// Đã có dòng `business_profile` (seed) ⇒ bootstrap UPDATE ĐÚNG dòng đó, không thêm dòng. Policy bảo
// mật (2FA/IP/trust-days) KHÔNG bị form ghi đè; field tuỳ chọn không gửi (currency, logo) giữ giá trị
// đang có; field tuỳ chọn có gửi (timezone) được ghi.
func TestBootstrap_ExistingProfile_UpdateSameRow_PolicyKept(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	resetOnboardingData(t)
	const uid = 924
	seedUser(t, uid, true)
	if err := testDB.Exec("INSERT INTO business_profile (id, name, code, timezone, currency, logo_image, " +
		"require_2fa, require_2fa_roles, allowed_ips, totp_trust_days) VALUES " +
		"(7, 'Seed', 'SEED', 'Asia/Bangkok', 'USD', 'seed.png', 1, '[\"admin\"]', '[\"10.0.0.0/8\"]', 7)").Error; err != nil {
		t.Fatalf("seed business_profile: %v", err)
	}

	profileJSON := fmt.Sprintf(`{"name":"Brand Upd","code":"UPD","business_type_id":%d,`+
		`"hotline":"0911111111","timezone":"Asia/Tokyo",`+
		`"allowed_ips":[],"require_2fa":0,"require_2fa_roles":[],"totp_trust_days":99}`, define.BusinessClinic)
	code, body := postBootstrap(t, newBootstrapRouter(t, uid, bootstrapRouterOpts{}), map[string]string{
		"company": profileJSON,
		"branch":  validBranchJSON("Branch Upd", "BUPD"),
	})
	if code != http.StatusCreated {
		t.Fatalf("HTTP = %d, muốn 201. Body: %s", code, body)
	}

	var n int64
	if err := testDB.Table("business_profile").Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("business_profile = %d dòng, muốn 1 (UPDATE dòng có sẵn, không INSERT)", n)
	}
	var bp struct {
		ID              uint   `gorm:"column:id"`
		Name            string `gorm:"column:name"`
		Code            string `gorm:"column:code"`
		Hotline         string `gorm:"column:hotline"`
		Timezone        string `gorm:"column:timezone"`
		Currency        string `gorm:"column:currency"`
		LogoImage       string `gorm:"column:logo_image"`
		Require2FA      int    `gorm:"column:require_2fa"`
		Require2FARoles string `gorm:"column:require_2fa_roles"`
		AllowedIPs      string `gorm:"column:allowed_ips"`
		TotpTrustDays   int    `gorm:"column:totp_trust_days"`
	}
	if err := testDB.Raw("SELECT id, name, code, hotline, timezone, currency, logo_image, require_2fa, " +
		"CAST(require_2fa_roles AS CHAR) AS require_2fa_roles, CAST(allowed_ips AS CHAR) AS allowed_ips, " +
		"totp_trust_days FROM business_profile").Scan(&bp).Error; err != nil {
		t.Fatalf("đọc business_profile: %v", err)
	}
	if bp.ID != 7 {
		t.Errorf("id = %d, muốn giữ dòng seed 7", bp.ID)
	}
	if bp.Name != "Brand Upd" || bp.Code != "UPD" || bp.Hotline != "0911111111" {
		t.Errorf("form không được ghi: %+v", bp)
	}
	if bp.Timezone != "Asia/Tokyo" {
		t.Errorf("timezone = %q, muốn Asia/Tokyo (form có gửi)", bp.Timezone)
	}
	if bp.Currency != "USD" || bp.LogoImage != "seed.png" {
		t.Errorf("currency=%q logo=%q — field tuỳ chọn không gửi phải giữ nguyên (USD / seed.png)", bp.Currency, bp.LogoImage)
	}
	if bp.Require2FA != 1 || bp.TotpTrustDays != 7 ||
		strings.ReplaceAll(bp.Require2FARoles, " ", "") != `["admin"]` ||
		strings.ReplaceAll(bp.AllowedIPs, " ", "") != `["10.0.0.0/8"]` {
		t.Errorf("policy bảo mật bị form ghi đè: require_2fa=%d roles=%s ips=%s trust=%d — muốn 1/[\"admin\"]/[\"10.0.0.0/8\"]/7",
			bp.Require2FA, bp.Require2FARoles, bp.AllowedIPs, bp.TotpTrustDays)
	}
}

func jsonArrayHas(raw string, id uint) bool {
	var ids []uint
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return false
	}
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// Story-58 (AC4): security_audit_log khoá theo event + user.
func requireAuditRow(t *testing.T, uid int) {
	t.Helper()
	deadline := time.Now().Add(6 * time.Second)
	for {
		var n int64
		if err := testDB.Table("security_audit_log").
			Where("event = ? AND user_id = ?", "onboarding_bootstrap", uid).
			Count(&n).Error; err != nil {
			t.Fatalf("đọc security_audit_log: %v", err)
		}
		if n == 1 {
			return
		}
		if n > 1 || time.Now().After(deadline) {
			t.Fatalf("security_audit_log onboarding_bootstrap user=%d: %d dòng, muốn 1", uid, n)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// ── AC4 ─────────────────────────────────────────────────────────────────────────────────────

// Đảo quyết định AC6 (2026-09-19): "bỏ qua chi nhánh" từng cho ra DB KHÔNG một chi nhánh nào
// ⇒ SeedConfigCodeForBranch chưa từng chạy ⇒ thiếu config_code, không tạo được dịch vụ đầu tiên (sự
// cố thật ARA Plus). Giờ thiếu branch phải bị từ chối 422 NGAY TẠI VALIDATE, trước cả named lock —
// 0 business_profile/branch/config_code được ghi, giống mọi lỗi validate khác trong bảng ở
// TestBootstrap_Validation_422_NoLock_NoChange (case "thiếu branch" cũng có ở đó).
func TestBootstrap_NoBranch_422_NoChange(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	resetOnboardingData(t)
	const uid = 915
	seedUser(t, uid, true)
	before := snapshot(t, uid)
	resetDexieVersions(t)

	code, body := postBootstrap(t, newBootstrapRouter(t, uid, bootstrapRouterOpts{}), map[string]string{
		"company": validProfileJSON("Brand AC4", "AC4"),
	})
	if code != http.StatusUnprocessableEntity || !strings.Contains(body, msg("onboarding_branch_required")) {
		t.Fatalf("HTTP = %d, muốn 422 onboarding_branch_required. Body: %s", code, body)
	}
	requireSnapshotEqual(t, before, snapshot(t, uid))
	if v := dexieVersions(t); len(v) != 0 {
		t.Errorf("dexieVersions = %v, KHÔNG được bump gì khi bootstrap bị từ chối", v)
	}
}

// ── AC5 ─────────────────────────────────────────────────────────────────────────────────────

// Lỗi GIỮA tx ⇒ rollback thật (0 business_profile/branch/config_code, user/profile không đổi).
//
// ⚠ Lệch story: AC5 gợi ý cấy "branch trùng tên" dựa trên tag `uniqueIndex` của entity, nhưng schema
// migrate THẬT không có unique index nào trên `branch.name`/`code` (chỉ PRIMARY + 2 KEY audit — đã
// kiểm information_schema.STATISTICS trên DB test) ⇒ trùng tên được INSERT bình thường, không gây lỗi.
// Dùng nhánh "lỗi giữa chừng" của AC5 bằng 2 lỗi thật ở 2 vị trí khác nhau trong tx.
func TestBootstrap_MidTxFailure_Rollback(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}

	// Lỗi ở BƯỚC CUỐI (không có user_profile) — sau khi business_profile + branch + config_code +
	// user.branchId đã ghi trong tx ⇒ phủ rollback của mọi bước trước.
	t.Run("last step: profile missing", func(t *testing.T) {
		resetOnboardingData(t)
		const uid = 916
		seedUser(t, uid, true)
		if err := testDB.Exec("DELETE FROM user_profile WHERE user_id = ?", uid).Error; err != nil {
			t.Fatalf("xoá profile: %v", err)
		}
		before := snapshot(t, uid)

		code, body := postBootstrap(t, newBootstrapRouter(t, uid, bootstrapRouterOpts{}), map[string]string{
			"company": validProfileJSON("Brand AC5a", "AC5A"),
			"branch":  validBranchJSON("Branch AC5a", "BAC5A"),
		})
		if code != http.StatusInternalServerError {
			t.Fatalf("HTTP = %d, muốn 500 khi user không có profile. Body: %s", code, body)
		}
		requireSnapshotEqual(t, before, snapshot(t, uid))
		// Khoá đã nhả cả khi tx lỗi.
		requireLockFree(t)
	})

	// Lỗi GIỮA seed config_code (bảng bị giấu ⇒ 1146) — sau khi business_profile + branch đã ghi.
	t.Run("mid: config_code seed fails", func(t *testing.T) {
		resetOnboardingData(t)
		const uid = 920
		seedUser(t, uid, true)
		before := snapshot(t, uid)

		if err := testDB.Exec("RENAME TABLE config_code TO config_code_onbwiz_hidden").Error; err != nil {
			t.Fatalf("không gây được lỗi seed config_code: %v", err)
		}
		restored := false
		restore := func() {
			if restored {
				return
			}
			restored = true
			if err := testDB.Exec("RENAME TABLE config_code_onbwiz_hidden TO config_code").Error; err != nil {
				t.Errorf("KHÔNG khôi phục được bảng config_code — test sau sẽ hỏng: %v", err)
			}
		}
		t.Cleanup(restore)

		code, body := postBootstrap(t, newBootstrapRouter(t, uid, bootstrapRouterOpts{}), map[string]string{
			"company": validProfileJSON("Brand AC5b", "AC5B"),
			"branch":  validBranchJSON("Branch AC5b", "BAC5B"),
		})
		restore() // trả bảng trước khi snapshot đếm config_code
		if code != http.StatusInternalServerError {
			t.Fatalf("HTTP = %d, muốn 500 khi seed config_code lỗi. Body: %s", code, body)
		}
		requireSnapshotEqual(t, before, snapshot(t, uid))
	})
}

// ── story-10 AC3 ────────────────────────────────────────────────────────────────────────────

// Nhánh lỗi của `userscope.AppendID` trong `Bootstrap` trước đây KHÔNG có test nào canh: mutation
// `_ = userscope.AppendID(...)` vẫn để test onboarding XANH (deferred-work.md, story-03 QA pass 1).
//
// Cột `user.branchId` chứa JSON hợp lệ nhưng KHÔNG phải mảng số ⇒ AppendID trả error (cố ý, để không
// âm thầm ghi đè mất quyền cũ) ⇒ tx rollback (sau khi business_profile + branch + config_code đã ghi
// trong tx) ⇒ 500, DB không đổi. (Story-62: khối AppendID thứ hai — cột thương hiệu — đã bỏ cùng cột.)
func TestBootstrap_AppendIDFails_Rollback(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	resetOnboardingData(t)
	const uid = 922
	seedUser(t, uid, true)
	if err := testDB.Exec("UPDATE `user` SET branchId = '\"corrupt\"' WHERE id = ?", uid).Error; err != nil {
		t.Fatalf("seed branchId hỏng: %v", err)
	}
	before := snapshot(t, uid)

	code, body := postBootstrap(t, newBootstrapRouter(t, uid, bootstrapRouterOpts{}), map[string]string{
		"company": validProfileJSON("Brand S10 branchId", "S10bra"),
		"branch":  validBranchJSON("Branch S10 branchId", "B10bra"),
	})
	if code != http.StatusInternalServerError {
		t.Fatalf("HTTP = %d, muốn 500 khi user.branchId không phải mảng id. Body: %s", code, body)
	}

	after := snapshot(t, uid)
	requireSnapshotEqual(t, before, after)
	// Nói rõ điều AC đòi: bảng hồ sơ + chi nhánh vẫn RỖNG (không chỉ "không đổi").
	if after.Profiles != 0 || after.Branches != 0 {
		t.Fatalf("business_profile=%d branch=%d sau rollback, muốn 0/0", after.Profiles, after.Branches)
	}
	// Cột giữ NGUYÊN giá trị hỏng — AppendID không được ghi đè thành `[id]`.
	if !after.UserBranchIDs.Valid || after.UserBranchIDs.String != `"corrupt"` {
		t.Fatalf("user.branchId bị ghi đè sau lỗi: %v", after.UserBranchIDs)
	}
	requireLockFree(t)
}

// ── AC8 ─────────────────────────────────────────────────────────────────────────────────────

// Validate đứng TRƯỚC khoá: giữ sẵn named lock ở connection khác — nếu handler cố lấy khoá sẽ chờ
// 5s rồi trả onboarding_in_progress, không phải thông điệp validate, và chậm.
func TestBootstrap_Validation_422_NoLock_NoChange(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	resetOnboardingData(t)
	const uid = 917
	seedUser(t, uid, true)
	holdLockElsewhere(t)
	r := newBootstrapRouter(t, uid, bootstrapRouterOpts{})
	before := snapshot(t, uid)

	cases := []struct {
		name   string
		fields map[string]string
		key    string
	}{
		{"form hồ sơ rỗng", map[string]string{}, "onboarding_invalid_payload"},
		{"form hồ sơ JSON hỏng", map[string]string{"company": "{"}, "onboarding_invalid_payload"},
		{"thiếu name", map[string]string{"company": `{"code":"X","business_type_id":1}`}, "onboarding_company_name_required"},
		{"thiếu code", map[string]string{"company": `{"name":"X","business_type_id":1}`}, "onboarding_company_code_required"},
		{"thiếu business_type_id", map[string]string{"company": `{"name":"X","code":"X"}`}, "onboarding_business_type_invalid"},
		{"business_type_id ngoài tập", map[string]string{"company": fmt.Sprintf(`{"name":"X","code":"X","business_type_id":%d}`, define.BusinessSpa+1)}, "onboarding_business_type_invalid"},
		{"thiếu branch (AC6 đảo 2026-09-19)", map[string]string{"company": validProfileJSON("X", "X")}, "onboarding_branch_required"},
		{"branch rỗng tường minh (null)", map[string]string{"company": validProfileJSON("X", "X"), "branch": "null"}, "onboarding_branch_required"},
		{"branch thiếu name", map[string]string{"company": validProfileJSON("X", "X"), "branch": `{"code":"B","address":"A"}`}, "onboarding_branch_name_required"},
		{"branch thiếu code", map[string]string{"company": validProfileJSON("X", "X"), "branch": `{"name":"B","address":"A"}`}, "onboarding_branch_code_required"},
		{"branch thiếu address", map[string]string{"company": validProfileJSON("X", "X"), "branch": `{"name":"B","code":"B"}`}, "onboarding_branch_address_required"},
		{"branch JSON hỏng", map[string]string{"company": validProfileJSON("X", "X"), "branch": "[1"}, "onboarding_invalid_payload"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			code, body := postBootstrap(t, r, tc.fields)
			if code != http.StatusUnprocessableEntity || !strings.Contains(body, msg(tc.key)) {
				t.Fatalf("HTTP = %d, muốn 422 %s. Body: %s", code, tc.key, body)
			}
			if el := time.Since(start); el > 2*time.Second {
				t.Errorf("mất %v — handler có vẻ đã chờ named lock trước khi validate", el)
			}
		})
	}
	requireSnapshotEqual(t, before, snapshot(t, uid))
}

// Khoá đang bị giữ quá thời gian chờ ⇒ 422 onboarding_in_progress, DB không đổi.
func TestBootstrap_LockHeld_422InProgress(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	resetOnboardingData(t)
	const uid = 918
	seedUser(t, uid, true)
	holdLockElsewhere(t)
	before := snapshot(t, uid)

	// Branch BẮT BUỘC kèm theo (2026-09-19) — thiếu branch giờ bị Validate chặn TRƯỚC khi chạm tới
	// khoá (xem TestBootstrap_Validation_422_NoLock_NoChange), nên test khoá phải gửi payload hợp
	// lệ đầy đủ để lỗi quan sát được đúng là do khoá, không phải do thiếu branch.
	code, body := postBootstrap(t, newBootstrapRouter(t, uid, bootstrapRouterOpts{}), map[string]string{
		"company": validProfileJSON("Brand Lock", "LOCK"),
		"branch":  validBranchJSON("Branch Lock", "BLOCK"),
	})
	if code != http.StatusUnprocessableEntity || !strings.Contains(body, msg("onboarding_in_progress")) {
		t.Fatalf("HTTP = %d, muốn 422 onboarding_in_progress. Body: %s", code, body)
	}
	requireSnapshotEqual(t, before, snapshot(t, uid))
}

// ── AC6 ─────────────────────────────────────────────────────────────────────────────────────

// 2 admin khác nhau, payload KHÁC tên/mã (để unique index của branch không tự cứu) bấm cùng lúc.
func TestBootstrap_Concurrent_ExactlyOneBranch(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	const rounds = 20
	const uidA, uidB = 921, 922
	rA := newBootstrapRouter(t, uidA, bootstrapRouterOpts{})
	rB := newBootstrapRouter(t, uidB, bootstrapRouterOpts{})

	for i := 0; i < rounds; i++ {
		t.Run(fmt.Sprintf("round-%02d", i), func(t *testing.T) {
			resetOnboardingData(t)
			seedUser(t, uidA, true)
			seedUser(t, uidB, true)

			var wg sync.WaitGroup
			var codeA, codeB int
			var bodyA, bodyB string
			start := make(chan struct{})
			wg.Add(2)
			go func() {
				defer wg.Done()
				<-start
				codeA, bodyA = postBootstrapNoFatal(rA, map[string]string{
					"company": validProfileJSON(fmt.Sprintf("Race A %d", i), fmt.Sprintf("RA%d", i)),
					"branch":  validBranchJSON(fmt.Sprintf("Race BA %d", i), fmt.Sprintf("RBA%d", i)),
				})
			}()
			go func() {
				defer wg.Done()
				<-start
				codeB, bodyB = postBootstrapNoFatal(rB, map[string]string{
					"company": validProfileJSON(fmt.Sprintf("Race B %d", i), fmt.Sprintf("RB%d", i)),
					"branch":  validBranchJSON(fmt.Sprintf("Race BB %d", i), fmt.Sprintf("RBB%d", i)),
				})
			}()
			close(start)
			wg.Wait()

			created, rejected := 0, 0
			for _, x := range []struct {
				code int
				body string
			}{{codeA, bodyA}, {codeB, bodyB}} {
				switch {
				case x.code == http.StatusCreated:
					created++
				case x.code == http.StatusUnprocessableEntity &&
					(strings.Contains(x.body, msg("onboarding_already_done")) || strings.Contains(x.body, msg("onboarding_in_progress"))):
					rejected++
				default:
					t.Errorf("response lạ: HTTP %d %s", x.code, x.body)
				}
			}
			var nBranch, nProfile int64
			if err := testDB.Table("branch").Count(&nBranch).Error; err != nil {
				t.Fatalf("đếm branch: %v", err)
			}
			if err := testDB.Table("business_profile").Count(&nProfile).Error; err != nil {
				t.Fatalf("đếm business_profile: %v", err)
			}
			if created != 1 || rejected != 1 || nBranch != 1 || nProfile != 1 {
				t.Fatalf("created=%d rejected=%d branch=%d business_profile=%d, muốn 1/1/1/1. A: %d %s | B: %d %s",
					created, rejected, nBranch, nProfile, codeA, bodyA, codeB, bodyB)
			}
		})
	}
}

// postBootstrapNoFatal — bản dùng trong goroutine (t.Fatalf chỉ hợp lệ trên goroutine của test).
func postBootstrapNoFatal(r *gin.Engine, fields map[string]string) (int, string) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			return -1, err.Error()
		}
	}
	if err := mw.Close(); err != nil {
		return -1, err.Error()
	}
	req := httptest.NewRequest(http.MethodPost, bootstrapPath, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code, w.Body.String()
}

// ── AC10 ────────────────────────────────────────────────────────────────────────────────────

// Chuỗi thật: AuthResponseHeaders + Redis (miniredis). Request đầu NẠP cache scope cũ (user chưa có
// chi nhánh). Bootstrap gán chi nhánh đầu cho user ⇒ scope cũ phải bị xoá (InvalidateUserScope).
//
// Story-58: gate bắt buộc chọn thương hiệu đã xoá ⇒ request đầu KHÔNG còn 403 (trước đây là tiền điều
// kiện của test). Bất biến giữ nguyên: cache scope có trước bootstrap và KHÔNG còn sau bootstrap.
func TestBootstrap_InvalidatesUserScope_GateOpens(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	resetOnboardingData(t)
	const uid = 919
	seedUser(t, uid, true)
	mr := miniredis.RunT(t)
	ch := cache.New(mr.Addr(), "", "", 0)
	r := newBootstrapRouter(t, uid, bootstrapRouterOpts{withAuthHeaders: true, cache: ch})
	scopeKey := fmt.Sprintf("user_scope:%d", uid)

	code, body := getPath(r, "/api/crm/customer")
	if code == http.StatusForbidden || code == http.StatusUnauthorized {
		t.Fatalf("tiền điều kiện: HTTP = %d — story-58 không còn gate chọn thương hiệu. Body: %s", code, body)
	}
	if !mr.Exists(scopeKey) {
		t.Fatalf("tiền điều kiện: %s chưa được cache — test không chứng minh được việc xoá cache", scopeKey)
	}

	code, body = postBootstrap(t, r, map[string]string{
		"company": validProfileJSON("Brand AC10", "AC10"),
		"branch":  validBranchJSON("Branch AC10", "BAC10"),
	})
	if code != http.StatusCreated {
		t.Fatalf("bootstrap HTTP = %d, muốn 201. Body: %s", code, body)
	}
	if mr.Exists(scopeKey) {
		t.Fatalf("%s còn trong cache sau bootstrap — InvalidateUserScope không chạy", scopeKey)
	}

	code, body = getPath(r, "/api/crm/customer")
	if code != http.StatusOK {
		t.Fatalf("request sau bootstrap HTTP = %d, muốn 200 (gate phải mở). Body: %s", code, body)
	}
}

// ── P3 (QA pass 1): tên khoá theo DB ────────────────────────────────────────────────────────

// Tên khoá thực tế chứa tên DB test; khoá của DB KHÁC (cùng instance MySQL) KHÔNG chặn bootstrap.
// Chiều ngược lại (khoá đúng tên của DB này thì chặn) do TestBootstrap_LockHeld_422InProgress phủ —
// cả hai cùng dựng tên bằng service.BootstrapLockName.
func TestBootstrap_LockNamePerDatabase(t *testing.T) {
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	var dbName string
	if err := testDB.Raw("SELECT DATABASE()").Row().Scan(&dbName); err != nil {
		t.Fatalf("DATABASE(): %v", err)
	}
	name := bootstrapLockName(t)
	if !strings.Contains(name, dbName) || !strings.HasPrefix(name, define.OnboardingBootstrapLock) {
		t.Fatalf("tên khoá %q không chứa tên DB %q", name, dbName)
	}

	resetOnboardingData(t)
	const uid = 923
	seedUser(t, uid, true)
	holdNamedLock(t, service.BootstrapLockName(dbName+"_khach_khac"))
	// Hằng trần (tên cũ, toàn server) cũng không được chặn nữa.
	holdNamedLock(t, define.OnboardingBootstrapLock)

	start := time.Now()
	// Branch BẮT BUỘC (2026-09-19) — payload phải hợp lệ đầy đủ để 201 quan sát được đúng là do
	// khoá KHÔNG chặn, không lẫn với 422 thiếu branch.
	code, body := postBootstrap(t, newBootstrapRouter(t, uid, bootstrapRouterOpts{}), map[string]string{
		"company": validProfileJSON("Brand P3", "P3"),
		"branch":  validBranchJSON("Branch P3", "BP3"),
	})
	if code != http.StatusCreated {
		t.Fatalf("HTTP = %d, muốn 201 — khoá của DB khác đang chặn bootstrap. Body: %s", code, body)
	}
	if el := time.Since(start); el > 3*time.Second {
		t.Errorf("mất %v — bootstrap đã chờ khoá của DB khác", el)
	}
}
