//go:build integration

package codegen

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"

	"erpCluvix/pkg/brand"
	"erpCluvix/pkg/define"
)

// Story-59 (E6/D22): chốt ĐỊNH DẠNG MÃ + BỘ ĐẾM cho từng loại mã. Story-62: expected GIỮ NGUYÊN khi
// chuyển nguồn tiền tố sang bảng `business_profile` (trước: bảng thương hiệu cũ).
//
// Hồ sơ doanh nghiệp: 1 dòng `business_profile` mã "CLX" (= brand.Code). Chi nhánh 11 "Q1", 12 "Q2" đã seed
// config_code theo mẫu SeedConfigCodeForBranch (service chỉ 1 row — dùng chung, không ghép chi
// nhánh, không reset tháng, 3 số); 13 "Q3" CHƯA seed. Seed COMMIT thật: bộ đếm mã khách chạy trên
// kết nối riêng (FK config_code_monthly→config_code chờ khoá nếu row chưa commit).

func pinGen(tx *gorm.DB, table string, branchID *uint) (string, error) {
	return GenerateErr(tx, table, branchID)
}

func pinCustomer(tx *gorm.DB, branchID int64) string {
	return NewCustomerCode(tx, branchID)
}

func pinFindConfig(tx *gorm.DB, table string, branchID *uint) *ConfigCode {
	return findConfig(tx, table, branchID)
}

// pinTables — infix theo defaultConfigCodeTemplates (config/branch/handler.SeedConfigCodeForBranch).
var pinTables = []struct{ table, infix string }{
	{"order_plan", "DH"},
	{"payment", "TT"},
	{"appointment", "LH"},
	{"reminder", "RE"},
	{"order_service_used", "SV"},
}

func seedPinTenant(t *testing.T) {
	t.Helper()
	if testDB == nil {
		t.Skip("cần TEST_MYSQL_DSN")
	}
	cleanup := func() {
		testDB.Exec("DELETE FROM config_code_monthly WHERE config_code_id IN (SELECT id FROM config_code WHERE branch_id IN (11, 12, 13))")
		testDB.Exec("DELETE FROM config_code WHERE branch_id IN (11, 12, 13)")
		testDB.Exec("DELETE FROM customer WHERE fullname = 'KH pin s59'")
		testDB.Exec("DELETE FROM branch WHERE id IN (11, 12, 13)")
		testDB.Exec("DELETE FROM business_profile")
		brand.Invalidate()
	}
	cleanup()
	qs := []string{
		"INSERT INTO business_profile (name, code) VALUES ('Thương hiệu pin', 'CLX')",
		"INSERT INTO branch (id, name, code, address) VALUES (11, 'Chi nhánh Q1', 'Q1', 'A'), (12, 'Chi nhánh Q2', 'Q2', 'B'), (13, 'Chi nhánh Q3', 'Q3', 'C')",
		`INSERT INTO config_code
			(branch_id, name, parent_table, infix_code, include_branch_code, monthly_reset, serial_length, created_at, updated_at)
			VALUES (11, 'Dịch vụ', 'service', 'DV', 0, 0, 3, 0, 0),
			       (11, 'Khách hàng', 'customer', 'KH', 1, 1, 2, 0, 0),
			       (12, 'Khách hàng', 'customer', 'KH', 1, 1, 2, 0, 0)`,
	}
	for _, b := range []int{11, 12} {
		for _, p := range pinTables {
			qs = append(qs, fmt.Sprintf(`INSERT INTO config_code
				(branch_id, name, parent_table, infix_code, include_branch_code, monthly_reset, serial_length, created_at, updated_at)
				VALUES (%d, '%s', '%s', '%s', 1, 1, 2, 0, 0)`, b, p.table, p.table, p.infix))
		}
	}
	for _, q := range qs {
		if err := testDB.Exec(q).Error; err != nil {
			t.Fatal(err)
		}
	}
	brand.Invalidate()
	t.Cleanup(cleanup)
}

func pinYM(t *testing.T) string {
	t.Helper()
	return time.Now().In(brand.Location(testDB)).Format("0601")
}

func mustPinGen(t *testing.T, tx *gorm.DB, table string, branchID *uint) string {
	t.Helper()
	code, err := pinGen(tx, table, branchID)
	if err != nil {
		t.Fatalf("Generate(%s): %v", table, err)
	}
	return code
}

// [P1] Định dạng mặc định BRAND.[BRANCH.]INFIX.[YYMM.]SERIAL từng loại mã + bộ đếm theo chi nhánh.
// Chi nhánh chưa seed / không chi nhánh ⇒ dùng row chi nhánh id nhỏ nhất (mã + bộ đếm của row đó).
func TestPinCodeFormatPerTable(t *testing.T) {
	seedPinTenant(t)
	ym := pinYM(t)
	b11, b12, b13 := uint(11), uint(12), uint(13)
	for _, p := range pinTables {
		t.Run(p.table, func(t *testing.T) {
			q1 := "CLX.Q1." + p.infix + "." + ym + "."
			q2 := "CLX.Q2." + p.infix + "." + ym + "."
			steps := []struct {
				branch *uint
				want   string
			}{
				{&b11, q1 + "01"},
				{&b11, q1 + "02"},
				{&b12, q2 + "01"}, // bộ đếm riêng chi nhánh
				{&b13, q1 + "03"}, // chưa seed ⇒ row chi nhánh 11
				{nil, q1 + "04"},  // không chi nhánh ⇒ row chi nhánh 11
				{&b12, q2 + "02"},
			}
			for i, s := range steps {
				if got := mustPinGen(t, testDB, p.table, s.branch); got != s.want {
					t.Fatalf("lần %d = %q, muốn %q", i+1, got, s.want)
				}
			}
		})
	}
	t.Run("service", func(t *testing.T) {
		for i, s := range []struct {
			branch *uint
			want   string
		}{{&b11, "CLX.DV.001"}, {&b12, "CLX.DV.002"}, {nil, "CLX.DV.003"}, {&b13, "CLX.DV.004"}} {
			if got := mustPinGen(t, testDB, "service", s.branch); got != s.want {
				t.Fatalf("lần %d = %q, muốn %q", i+1, got, s.want)
			}
		}
	})
	// config_code chưa seed bảng ⇒ ("", nil), không phải lỗi.
	if code, err := pinGen(testDB, "bang_chua_seed_s59", &b11); code != "" || err != nil {
		t.Fatalf("bảng chưa seed = (%q, %v), muốn (\"\", nil)", code, err)
	}
}

// [P2] Mã khách theo từng CUSTOMER_CODE_OPTION trên thương hiệu pin (chi nhánh 11, 11, 12, 13).
func TestPinCustomerCodeFormats(t *testing.T) {
	cases := []struct {
		option string
		want   func(ym string) []string
	}{
		{"", func(ym string) []string {
			return []string{"CLX.Q1.KH." + ym + ".01", "CLX.Q1.KH." + ym + ".02", "CLX.Q2.KH." + ym + ".01", "CLX.Q3.KH." + ym + ".03"}
		}},
		{"2", func(ym string) []string {
			return []string{"Q1." + ym + ".01", "Q1." + ym + ".02", "Q2." + ym + ".01", "Q3." + ym + ".03"}
		}},
		{"3", func(string) []string { return []string{"Q1.01", "Q1.02", "Q2.01", "Q3.03"} }},
		{"4", func(string) []string { return []string{"11.00001", "11.00002", "12.00001", "13.00003"} }},
		{"5", func(string) []string { return []string{"KH.00001", "KH.00002", "KH.00003", "KH.00004"} }},
		{"6", func(ym string) []string {
			return []string{"CLX.KH." + ym + ".001", "CLX.KH." + ym + ".002", "CLX.KH." + ym + ".003", "CLX.KH." + ym + ".004"}
		}},
	}
	for _, c := range cases {
		t.Run("option="+c.option, func(t *testing.T) {
			seedPinTenant(t)
			t.Setenv(define.CustomerCodeOptionEnv, c.option)
			want := c.want(pinYM(t))
			for i, b := range []int64{11, 11, 12, 13} {
				if got := pinCustomer(testDB, b); got != want[i] {
					t.Fatalf("lần %d (chi nhánh %d) = %q, muốn %q", i+1, b, got, want[i])
				}
			}
		})
	}
}

// [P3] AC4: 2 chi nhánh + option đếm chung (6) ⇒ MỘT bộ đếm chung, tiền tố = brand.Profile().Code.
func TestPinSharedCounterUsesBrandCode(t *testing.T) {
	seedPinTenant(t)
	t.Setenv(define.CustomerCodeOptionEnv, "6")
	prefix := brand.Profile(testDB).Code
	if prefix != "CLX" {
		t.Fatalf("brand.Profile().Code = %q, muốn CLX", prefix)
	}
	ym := pinYM(t)
	for i, b := range []int64{12, 11, 12} {
		want := fmt.Sprintf("%s.KH.%s.%03d", prefix, ym, i+1)
		if got := pinCustomer(testDB, b); got != want {
			t.Fatalf("lần %d (chi nhánh %d) = %q, muốn %q", i+1, b, got, want)
		}
	}
	// Đổi mã thương hiệu ⇒ tiền tố đổi theo (sau Invalidate), bộ đếm KHÔNG về 1.
	if err := testDB.Exec("UPDATE business_profile SET code = 'NEW'").Error; err != nil {
		t.Fatal(err)
	}
	brand.Invalidate()
	if got, want := pinCustomer(testDB, 11), "NEW.KH."+ym+".004"; got != want {
		t.Fatalf("sau đổi mã thương hiệu = %q, muốn %q", got, want)
	}
}

// [P4] Chọn row config_code TẤT ĐỊNH theo chi nhánh: schema mới UNIQUE (branch_id, parent_table) ⇒
// mỗi chi nhánh tối đa 1 row. Không chi nhánh / chi nhánh chưa seed ⇒ row của chi nhánh id NHỎ NHẤT,
// KHÔNG phụ thuộc row nào chèn trước (row chi nhánh 12 chèn trước, id nhỏ hơn). Gọi lặp lại luôn cùng
// row ⇒ bộ đếm nối tiếp, không nhảy/trùng. (Trước story-62: ca "nhiều row của nhiều thương hiệu cùng
// chi nhánh" — schema mới không còn cho phép.)
func TestPinMultiBranchRowsDeterministic(t *testing.T) {
	seedPinTenant(t)
	for _, q := range []string{
		`INSERT INTO config_code (branch_id, name, parent_table, infix_code, include_branch_code, monthly_reset, serial_length, created_at, updated_at)
			VALUES (12, 'X', 'pin_multi', 'NN', 1, 0, 2, 0, 0)`,
		`INSERT INTO config_code (branch_id, name, parent_table, infix_code, include_branch_code, monthly_reset, serial_length, created_at, updated_at)
			VALUES (11, 'X', 'pin_multi', 'MM', 1, 0, 2, 0, 0)`,
	} {
		if err := testDB.Exec(q).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := testDB.Exec(`INSERT INTO config_code (branch_id, name, parent_table, infix_code, include_branch_code, monthly_reset, serial_length, created_at, updated_at)
			VALUES (11, 'X', 'pin_multi', 'ZZ', 1, 0, 2, 0, 0)`).Error; err == nil {
		t.Fatal("UNIQUE (branch_id, parent_table) phải chặn row thứ hai cùng chi nhánh + cùng bảng")
	}
	b13 := uint(13)
	if cfg := pinFindConfig(testDB, "pin_multi", &b13); cfg == nil || cfg.InfixCode != "MM" {
		t.Fatalf("findConfig(chi nhánh 13 chưa seed) = %+v, muốn row chi nhánh 11 / MM", cfg)
	}
	b11 := uint(11)
	cfg := pinFindConfig(testDB, "pin_multi", &b11)
	if cfg == nil || cfg.InfixCode != "MM" || cfg.BranchID == nil || *cfg.BranchID != 11 {
		t.Fatalf("findConfig(chi nhánh 11) = %+v, muốn row chi nhánh 11 / MM", cfg)
	}
	noBranch := pinFindConfig(testDB, "pin_multi", nil)
	if noBranch == nil || noBranch.InfixCode != "MM" || noBranch.BranchID == nil || *noBranch.BranchID != 11 {
		t.Fatalf("findConfig(không chi nhánh) = %+v, muốn row chi nhánh 11 (chi nhánh nhỏ nhất)", noBranch)
	}
	for i, want := range []string{"CLX.Q1.MM.01", "CLX.Q1.MM.02", "CLX.Q1.MM.03"} {
		var bp *uint
		if i == 2 {
			bp = nil
		} else {
			bp = &b11
		}
		if got := mustPinGen(t, testDB, "pin_multi", bp); got != want {
			t.Fatalf("lần %d = %q, muốn %q", i+1, got, want)
		}
	}
}

// [P5] Đồng thời: nhiều request (mỗi request 1 tx, như hook BeforeCreate) sinh mã cùng bảng + cùng chi
// nhánh ⇒ không trùng, serial liền mạch 1..N. Khách hàng (bộ đếm kết nối riêng / lùi về tx) cũng vậy.
func TestPinConcurrentNoDuplicate(t *testing.T) {
	seedPinTenant(t)
	t.Setenv(define.CustomerCodeOptionEnv, "3")
	const workers, per = 8, 5
	b11 := uint(11)

	run := func(name string, gen func(tx *gorm.DB) string) []string {
		var mu sync.Mutex
		var codes []string
		var errs []string
		var wg sync.WaitGroup
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < per; i++ {
					tx := testDB.Begin()
					code := gen(tx)
					if err := tx.Commit().Error; err != nil {
						mu.Lock()
						errs = append(errs, err.Error())
						mu.Unlock()
						continue
					}
					mu.Lock()
					codes = append(codes, code)
					mu.Unlock()
				}
			}()
		}
		wg.Wait()
		if len(errs) > 0 {
			t.Fatalf("%s: commit lỗi: %v", name, errs)
		}
		return codes
	}

	// Số 1 của kỳ sinh trước (tuần tự): lượt ĐẦU kỳ chèn dòng bộ đếm — khe 2 kết nối cùng chèn đã
	// được ghi nhận/chấp nhận ở bumpSerial; test này chốt đường thường (dòng bộ đếm đã có).
	check := func(name string, codes []string, want func(n int) string) {
		t.Helper()
		if len(codes) != workers*per {
			t.Fatalf("%s: %d mã, muốn %d", name, len(codes), workers*per)
		}
		sort.Strings(codes)
		wantAll := make([]string, 0, workers*per)
		for n := 2; n <= workers*per+1; n++ {
			wantAll = append(wantAll, want(n))
		}
		sort.Strings(wantAll)
		if strings.Join(codes, ",") != strings.Join(wantAll, ",") {
			t.Fatalf("%s: mã sinh song song\n  có:   %v\n  muốn: %v", name, codes, wantAll)
		}
	}

	ym := pinYM(t)
	if got, want := mustPinGen(t, testDB, "order_plan", &b11), "CLX.Q1.DH."+ym+".01"; got != want {
		t.Fatalf("mã đơn đầu kỳ = %q, muốn %q", got, want)
	}
	if got := pinCustomer(testDB, 11); got != "Q1.01" {
		t.Fatalf("mã khách đầu = %q, muốn Q1.01", got)
	}
	orders := run("order_plan", func(tx *gorm.DB) string {
		code, err := pinGen(tx, "order_plan", &b11)
		if err != nil {
			return "ERR:" + err.Error()
		}
		return code
	})
	check("order_plan", orders, func(n int) string { return fmt.Sprintf("CLX.Q1.DH.%s.%02d", ym, n) })

	customers := run("customer", func(tx *gorm.DB) string { return pinCustomer(tx, 11) })
	check("customer", customers, func(n int) string { return fmt.Sprintf("Q1.%02d", n) })
}

// [P6] Caller gọi codegen bằng db THƯỜNG, không transaction (crm/reminder/handler/upsert.go:
// `codegen.Generate(h.DB, …)`): UPDATE … LAST_INSERT_ID(…) và SELECT LAST_INSERT_ID() phải chạy trên
// CÙNG 1 kết nối — nextSerial tự bọc db.Transaction để ghim kết nối. Pool nhiều kết nối + 20 goroutine
// ⇒ bỏ lớp bọc đó là đọc nhầm số của kết nối khác ⇒ trùng/nhảy mã. (Các test P1–P5 gọi trong tx sẵn có
// nên tx đã tự ghim kết nối — không bắt được lỗi này; QA-C story-59.)
func TestPinConcurrentPlainDBNoDuplicate(t *testing.T) {
	seedPinTenant(t)
	sqlDB, err := testDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	prevMax := sqlDB.Stats().MaxOpenConnections
	sqlDB.SetMaxOpenConns(8)
	sqlDB.SetMaxIdleConns(8)
	t.Cleanup(func() { sqlDB.SetMaxOpenConns(prevMax) })

	const workers, per = 20, 3
	b11, b12 := uint(11), uint(12)
	ym := pinYM(t)
	// Số 1 của kỳ sinh tuần tự: khe lượt ĐẦU kỳ (2 kết nối cùng chèn dòng bộ đếm) đã được chấp nhận ở
	// bumpSerial — test này chốt bất biến "cùng 1 kết nối".
	for _, b := range []*uint{&b11, &b12} {
		if _, err := GenerateErr(testDB, "reminder", b); err != nil {
			t.Fatalf("warm-up: %v", err)
		}
	}

	var mu sync.Mutex
	var codes, errs []string
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		bid := &b11
		if w%2 == 0 {
			bid = &b12
		}
		wg.Add(1)
		go func(bid *uint) {
			defer wg.Done()
			for i := 0; i < per; i++ {
				code, err := GenerateErr(testDB, "reminder", bid) // db thường, KHÔNG tx
				mu.Lock()
				if err != nil {
					errs = append(errs, err.Error())
				} else {
					codes = append(codes, code)
				}
				mu.Unlock()
			}
		}(bid)
	}
	wg.Wait()
	if len(errs) > 0 {
		t.Fatalf("lỗi bộ đếm: %v", errs)
	}

	var want []string
	for _, branch := range []string{"Q1", "Q2"} {
		for n := 2; n <= workers/2*per+1; n++ {
			want = append(want, fmt.Sprintf("CLX.%s.RE.%s.%02d", branch, ym, n))
		}
	}
	sort.Strings(codes)
	sort.Strings(want)
	if strings.Join(codes, ",") != strings.Join(want, ",") {
		t.Fatalf("mã sinh song song trên db thường (trùng/nhảy số)\n  có:   %v\n  muốn: %v", codes, want)
	}
}
