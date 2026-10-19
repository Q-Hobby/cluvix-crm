package codegen

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"erpCluvix/pkg/brand"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/logger"

	"gorm.io/gorm"
)

// Mã khách hàng — quy tắc TOÀN DỰ ÁN: khách CHƯA có chi nhánh thì CHƯA có mã (khách được lưu từ
// lúc còn là Lead, chưa biết thuộc chi nhánh nào); có chi nhánh thì sinh mã ngay. Mọi đường tạo/gán
// chi nhánh cho khách PHẢI đi qua NewCustomerCode/EnsureCustomerCode — KHÔNG gọi thẳng
// Generate(tx, "customer", ...), vì Generate không biết quy tắc này lẫn kiểu mã đang chọn.
//
// Kiểu mã chọn bằng env CUSTOMER_CODE_OPTION (bảng ví dụ: pkg/define/customer.go). Cả 2 hàm trả ""
// khi không sinh được mã (chưa có chi nhánh, khách đã có mã, config_code chưa seed) ⇒ caller chỉ
// gán cột `code` khi kết quả khác "".

// NewCustomerCode sinh mã cho khách SẮP TẠO. branchID <= 0 nghĩa là không có (nhận int64 để caller
// truyền thẳng helpers.ToInt64 — ép giá trị âm sang uint sẽ ra id rác mà vẫn sinh mã).
// Sinh nhiều mã trong 1 request (import) ⇒ dùng NewCustomerCoder để cache config/mã chi nhánh.
func NewCustomerCode(tx *gorm.DB, branchID int64) string {
	return NewCustomerCoder(tx).New(branchID)
}

// EnsureCustomerCode sinh mã cho khách ĐÃ TỒN TẠI nếu khách chưa có mã. branchID là chi nhánh khách
// đang/sắp thuộc về (0 ⇒ không sinh).
func EnsureCustomerCode(tx *gorm.DB, customerID, branchID int64) string {
	return NewCustomerCoder(tx).Ensure(customerID, branchID)
}

// CustomerCoder sinh mã khách trong phạm vi 1 request: đọc option 1 lần, cache row config_code và mã
// chi nhánh theo khoá ⇒ import N dòng không lặp lại N lần các câu tra cứu đó. KHÔNG giữ qua request
// (config sửa được trên màn Cấu hình mã).
type CustomerCoder struct {
	tx          *gorm.DB
	format      customerCodeFormat
	custom      bool                 // false ⇒ option mặc định: định dạng lấy từ config_code
	configs     map[uint]*ConfigCode // khoá = chi nhánh; 0 ⇒ row đếm chung toàn hệ thống
	branchCodes map[uint]string
	// inTx — đã có 1 lượt phải lùi về đếm trong tx (nextSerial) ⇒ tx đang giữ khoá dòng bộ đếm tới
	// commit ⇒ các lượt sau của CÙNG coder đếm luôn trong tx. Mở kết nối riêng lúc này là chờ chính tx
	// của mình tới hết hạn (import: mỗi dòng còn lại mất trọn customerSerialLockWait).
	inTx bool
}

func NewCustomerCoder(tx *gorm.DB) *CustomerCoder {
	format, custom := customerCodeFormats[customerCodeOption()]
	return &CustomerCoder{
		tx:          tx,
		format:      format,
		custom:      custom,
		configs:     map[uint]*ConfigCode{},
		branchCodes: map[uint]string{},
	}
}

// New — xem NewCustomerCode.
func (cc *CustomerCoder) New(branchID int64) string {
	if branchID <= 0 {
		return ""
	}
	for range customerCodeMaxAttempts {
		code := cc.generate(uint(branchID))
		if code == "" || !customerCodeTaken(cc.tx, code) {
			return code
		}
	}
	if logger.S != nil {
		logger.S.Errorf("codegen: %d mã khách hàng liên tiếp đã tồn tại (branch=%d) — bỏ trống mã",
			customerCodeMaxAttempts, branchID)
	}
	return ""
}

// Ensure — xem EnsureCustomerCode.
func (cc *CustomerCoder) Ensure(customerID, branchID int64) string {
	if customerID <= 0 || branchID <= 0 {
		return ""
	}
	var cust struct {
		Code string `gorm:"column:code"`
	}
	if err := cc.tx.Table("customer").Select("COALESCE(code, '') AS code").
		Where("id = ?", customerID).Scan(&cust).Error; err != nil {
		if logger.S != nil {
			logger.S.Errorf("codegen: đọc khách %d để sinh mã lỗi (%v) — bỏ trống mã", customerID, err)
		}
		return ""
	}
	if cust.Code != "" {
		return ""
	}
	code := cc.New(branchID)
	if code == "" {
		return ""
	}
	// Giành mã có điều kiện: bước đọc ở trên là đọc nhất quán (ảnh chụp, không khoá) ⇒ 2 request đồng
	// thời (tạo đơn + tạo lịch hẹn cho cùng khách chưa mã) đều thấy rỗng, đều sinh mã, UPDATE sau ghi đè
	// mã request trước đã trả/hiển thị. UPDATE là đọc có khoá, thấy bản mới nhất ⇒ bên thua nhận 0 dòng,
	// trả "" để caller KHÔNG ghi đè (số đã cấp bị bỏ qua — như tx rollback).
	res := cc.tx.Exec("UPDATE customer SET code = ? WHERE id = ? AND (code IS NULL OR code = '')", code, customerID)
	if res.Error != nil {
		if logger.S != nil {
			logger.S.Errorf("codegen: gán mã %q cho khách %d lỗi (%v) — bỏ trống mã", code, customerID, res.Error)
		}
		return ""
	}
	if res.RowsAffected != 1 {
		return ""
	}
	return code
}

// customerCodeMaxAttempts — số lần nhảy số tối đa khi mã sinh ra đã có người dùng. Trùng xảy ra khi
// đổi CUSTOMER_CODE_OPTION / tắt mã chi nhánh trong config_code giữa chừng; mỗi lần thử đã tăng bộ
// đếm nên chỉ vướng đúng những số đã cấp trước đó, 50 là dư.
const customerCodeMaxAttempts = 50

// customerCodeTaken — mã đã có khách nào dùng chưa (toàn DB — 1 thương hiệu/DB, D23: UNIQUE(code)).
// Khớp UNIQUE(code) của schema mới (tên index giữ lịch sử `uniq_customer_code`).
func customerCodeTaken(tx *gorm.DB, code string) bool {
	var n int64
	tx.Table("customer").Where("code = ?", code).Limit(1).Count(&n)
	return n > 0
}

// customerCodeFormat — định dạng 1 option. Bộ đếm là `config_code_monthly` của row config_code
// "customer":
//   - shared = false ⇒ row của CHÍNH chi nhánh ⇒ đếm riêng từng chi nhánh. Mã PHẢI chứa chi
//     nhánh (mã/id), nếu không 2 chi nhánh ra cùng một mã.
//   - shared = true ⇒ row của chi nhánh id nhỏ nhất (findConfig bước 2) ⇒ 1 bộ đếm chung toàn hệ
//     thống. Row đó ổn định: FK config_code→branch là RESTRICT nên chi nhánh không bị xoá.
//
// Đổi option giữa chừng vẫn dùng chung bộ đếm ⇒ số thứ tự chỉ tăng tiếp, không quay về 1 ⇒ không đụng
// mã đã cấp theo option trước.
type customerCodeFormat struct {
	shared      bool // 1 bộ đếm chung toàn hệ thống (không theo chi nhánh)
	monthly     bool // kèm YYMM + bộ đếm theo tháng
	serialWidth int  // độ dài tối thiểu, tự nở khi vượt
	branch      branchPart
	infix       bool // ghép infix_code của config_code (mặc định "KH")
	brandCode   bool // ghép mã thương hiệu (brand.Profile().Code — D22)
}

type branchPart int

const (
	branchNone        branchPart = iota
	branchCode                   // branch.code, rỗng thì dùng id để 2 chi nhánh không trùng mã
	branchCodeOrEmpty            // branch.code, rỗng thì bỏ (định dạng option mặc định từ trước tới nay)
	branchID
)

var customerCodeFormats = map[int]customerCodeFormat{
	define.CustomerCodeOptionBranchMonth:     {branch: branchCode, monthly: true, serialWidth: 2},
	define.CustomerCodeOptionBranch:          {branch: branchCode, serialWidth: 2},
	define.CustomerCodeOptionBranchID:        {branch: branchID, serialWidth: 5},
	define.CustomerCodeOptionInfix:           {shared: true, infix: true, serialWidth: 5},
	define.CustomerCodeOptionBrandInfixMonth: {shared: true, brandCode: true, infix: true, monthly: true, serialWidth: 3},
}

func (cc *CustomerCoder) generate(bid uint) string {
	f := cc.format
	var cfg *ConfigCode
	if cc.custom && f.shared {
		cfg = cc.config(nil)
	} else {
		cfg = cc.config(&bid)
	}
	if cfg == nil {
		return ""
	}
	if !cc.custom {
		// Option mặc định: định dạng theo màn Cấu hình mã — BRAND.[BRANCH.]INFIX.[YYMM.]SERIAL.
		f = customerCodeFormat{brandCode: true, infix: true, monthly: cfg.MonthlyReset, serialWidth: cfg.SerialLength}
		if cfg.IncludeBranchCode {
			f.branch = branchCodeOrEmpty
		}
	}

	var parts []string
	if f.brandCode {
		// ProfileStrict: lỗi DB ⇒ bỏ trống mã, KHÔNG sinh mã thiếu tiền tố thương hiệu.
		prof, err := brand.ProfileStrict(cc.tx)
		if err != nil {
			if logger.S != nil {
				logger.S.Errorf("codegen: đọc mã thương hiệu lỗi (%v) — bỏ trống mã khách", err)
			}
			return ""
		}
		parts = append(parts, prof.Code)
	}
	switch f.branch {
	case branchCode, branchCodeOrEmpty:
		// Mã của CHÍNH chi nhánh khách, KHÔNG phải cfg.BranchCode — cfg có thể là row của chi nhánh
		// khác (findConfig bước 2 khi chi nhánh này chưa seed).
		code := cc.branchCode(bid)
		if code == "" && f.branch == branchCode {
			code = strconv.FormatUint(uint64(bid), 10)
		}
		parts = append(parts, code)
	case branchID:
		parts = append(parts, strconv.FormatUint(uint64(bid), 10))
	}
	if f.infix {
		parts = append(parts, strings.TrimRight(cfg.InfixCode, "."))
	}

	serial, ym, err := cc.nextSerial(cfg.ID, f.monthly)
	if err != nil {
		return "" // đã log trong nextSerial — KHÔNG bịa số
	}
	return joinCode(parts, ym, serial, f.serialWidth)
}

func (cc *CustomerCoder) config(branchID *uint) *ConfigCode {
	key := uint(0)
	if branchID != nil {
		key = *branchID
	}
	if cfg, ok := cc.configs[key]; ok {
		return cfg // nil cũng cache: chưa seed thì không tra lại mỗi dòng
	}
	cfg := findConfig(cc.tx, "customer", branchID)
	cc.configs[key] = cfg
	return cfg
}

func (cc *CustomerCoder) branchCode(bid uint) string {
	if code, ok := cc.branchCodes[bid]; ok {
		return code
	}
	var code string
	cc.tx.Table("branch").Select("COALESCE(code, '')").Where("id = ?", bid).Scan(&code)
	cc.branchCodes[bid] = code
	return code
}

// nextSerial tăng bộ đếm mã khách trên KẾT NỐI RIÊNG, autocommit — tách khỏi transaction nghiệp vụ
// của caller (user chốt 2026-09-21, chỉ cho mã khách). Lý do: trong tx, lượt tăng giữ khoá dòng
// `config_code_monthly` tới lúc commit ⇒ import vài nghìn dòng khoá bộ đếm của chi nhánh (option 5/6:
// bộ đếm chung) suốt lần import, người khác tạo khách chờ tới lock-wait-timeout. Đổi lại: tx nghiệp vụ
// rollback thì số đã cấp bị bỏ qua (vốn đã chấp nhận). Tăng + LAST_INSERT_ID trên CÙNG 1 conn (bumpSerial).
//
// ⚠ Row config_code phải đã COMMIT: FK config_code_monthly→config_code khiến kết nối riêng chờ khoá
// nếu row do chính tx caller vừa tạo. ⚠ Caller đang giữ 1 kết nối (tx) mà xin thêm kết nối thứ 2 ⇒ pool
// có trần (test: SetMaxOpenConns(1)) cạn là chờ vĩnh viễn ⇒ chờ lấy kết nối có hạn
// customerSerialConnWait, chờ khoá có hạn customerSerialLockWait. Mọi lỗi ⇒ lùi về đếm trong tx và nhớ
// lại (cc.inTx). Đếm trong tx cũng lỗi ⇒ trả lỗi, caller bỏ trống mã.
func (cc *CustomerCoder) nextSerial(configCodeID uint, monthly bool) (int64, string, error) {
	if !cc.inTx {
		year, month, ym := serialPeriod(cc.tx, monthly)
		serial, err := upsertSerialOwnConn(cc.tx, configCodeID, year, month)
		if err == nil {
			return serial, ym, nil
		}
		if !errors.Is(err, errPoolFull) && logger.S != nil {
			logger.S.Errorf("codegen: bộ đếm mã khách trên kết nối riêng lỗi (%v) — đếm trong transaction", err)
		}
		cc.inTx = true
	}
	serial, ym, err := nextSerial(cc.tx, configCodeID, monthly)
	if err != nil && logger.S != nil {
		logger.S.Errorf("codegen: bộ đếm mã khách (config_code %d) lỗi cả trong transaction (%v) — bỏ trống mã", configCodeID, err)
	}
	return serial, ym, err
}

var errPoolFull = errors.New("pool kết nối đã kín")

const (
	// customerSerialConnWait — thời gian chờ tối đa để lấy kết nối riêng cho bộ đếm.
	customerSerialConnWait = 3 * time.Second
	// customerSerialLockWait (giây) — innodb_lock_wait_timeout của kết nối riêng. Mặc định server là 50s:
	// kết nối riêng mà chờ khoá do tx khác (hoặc chính tx caller) giữ thì request treo trọn 50s rồi mới
	// lùi về tx (sự cố 2026-09-22, tạo đơn 51s). Lượt tăng bình thường chỉ vài ms.
	customerSerialLockWait = 5
)

func upsertSerialOwnConn(db *gorm.DB, configCodeID uint, year, month int) (serial int64, err error) {
	sqlDB, err := db.DB()
	if err != nil {
		return 0, err
	}
	ctx := context.Background()
	if db.Statement != nil && db.Statement.Context != nil {
		ctx = db.Statement.Context
	}
	if st := sqlDB.Stats(); st.MaxOpenConnections > 0 && st.InUse >= st.MaxOpenConnections {
		return 0, errPoolFull // pool có trần và đã kín — chờ thêm chỉ tốn thời gian
	}
	acquireCtx, cancel := context.WithTimeout(ctx, customerSerialConnWait)
	conn, err := sqlDB.Conn(acquireCtx)
	cancel()
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	// Biến session sống theo kết nối, mà kết nối quay về pool dùng chung ⇒ PHẢI trả về DEFAULT trước khi
	// Close. Trả không được ⇒ huỷ hẳn kết nối (driver.ErrBadConn trong Raw), không để request khác nhận
	// một kết nối chỉ chờ khoá 5s. Dùng context.Background: ctx request đã huỷ vẫn phải reset được.
	if _, err := conn.ExecContext(ctx, "SET SESSION innodb_lock_wait_timeout = ?", customerSerialLockWait); err != nil {
		return 0, err
	}
	defer func() {
		if _, rerr := conn.ExecContext(context.Background(), "SET SESSION innodb_lock_wait_timeout = DEFAULT"); rerr != nil {
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
	}()

	serial, err = bumpSerial(func(query string, args ...any) (int64, error) {
		res, err := conn.ExecContext(ctx, query, args...)
		if err != nil {
			return 0, err
		}
		return res.RowsAffected()
	}, func() (id int64, err error) {
		err = conn.QueryRowContext(ctx, "SELECT LAST_INSERT_ID()").Scan(&id)
		return id, err
	}, configCodeID, year, month)
	if err != nil {
		return 0, err
	}
	if serial <= 0 {
		return 0, fmt.Errorf("bộ đếm trả số không hợp lệ %d", serial)
	}
	return serial, nil
}

var warnBadOption sync.Once

// CustomerCodeOption — kiểu mã khách đang áp dụng (đã chuẩn hoá: không đặt / sai ⇒ mặc định). Cho màn
// Cấu hình mã biết cấu hình dòng "customer" có còn quyết định định dạng mã hay không.
func CustomerCodeOption() int { return customerCodeOption() }

// customerCodeOption đọc CUSTOMER_CODE_OPTION. Không đặt ⇒ mặc định; giá trị không hợp lệ ⇒ mặc
// định + báo lỗi 1 lần/process (cấu hình sai phải có người thấy, nhưng không được chặn tạo khách).
func customerCodeOption() int {
	raw := strings.TrimSpace(os.Getenv(define.CustomerCodeOptionEnv))
	if raw == "" {
		return define.CustomerCodeOptionDefault
	}
	opt, err := strconv.Atoi(raw)
	if err == nil {
		if _, ok := customerCodeFormats[opt]; ok || opt == define.CustomerCodeOptionDefault {
			return opt
		}
	}
	warnBadOption.Do(func() {
		if logger.S != nil {
			logger.S.Errorf("%s=%q không hợp lệ — dùng kiểu mã mặc định %d",
				define.CustomerCodeOptionEnv, raw, define.CustomerCodeOptionDefault)
		}
	})
	return define.CustomerCodeOptionDefault
}
