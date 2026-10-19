package codegen

import (
	"fmt"
	"strings"
	"time"

	"erpCluvix/pkg/brand"
	"erpCluvix/pkg/logger"

	"gorm.io/gorm"
)

// ConfigCode tương đương common\models\ConfigCode. Tiền tố thương hiệu lấy từ pkg/brand
// (`business_profile.code`).
type ConfigCode struct {
	ID                uint   `gorm:"primaryKey" form:"-"`
	BranchID          *uint  `gorm:"column:branch_id"`
	ParentTable       string `gorm:"column:parent_table"`
	InfixCode         string `gorm:"column:infix_code"`
	SerialLength      int    `gorm:"column:serial_length"`
	IncludeBranchCode bool   `gorm:"column:include_branch_code"`
	MonthlyReset      bool   `gorm:"column:monthly_reset"`
	BranchCode        string `gorm:"column:branch_code;->"` // loaded via join
}

func (ConfigCode) TableName() string { return "config_code" }

// Generate tạo mã tự động cho record mới. Trả "" khi config_code chưa seed bảng này HOẶC bộ đếm lỗi
// (lỗi đã log) ⇒ caller PHẢI coi "" là không có mã. Caller không có nhánh xử lý "" ⇒ dùng GenerateErr.
// Tương đương MyModel::setCode() + generateMonthly().
func Generate(db *gorm.DB, table string, branchID *uint) string {
	code, _ := GenerateErr(db, table, branchID)
	return code
}

// GenerateErr như Generate nhưng trả lỗi bộ đếm (lock-wait, deadlock, mất kết nối) để caller rollback.
// config_code chưa seed KHÔNG phải lỗi: trả ("", nil) như Generate — giữ nguyên hành vi cũ của các luồng
// chấp nhận mã rỗng.
//
// ⛔ KHÔNG được bịa số khi bộ đếm lỗi (bản cũ trả serial 1 ⇒ mã ".01" trùng mã đã cấp: bảng có UNIQUE
// thì request vỡ 1062, bảng không UNIQUE — journal_entry.entry_no, invoice.draft_code — thì TRÙNG SỐ
// CHỨNG TỪ im lặng). Deadlock 1213 còn đã rollback cả tx của caller: chạy tiếp là ghi dở dang.
func GenerateErr(db *gorm.DB, table string, branchID *uint) (string, error) {
	cfg := findConfig(db, table, branchID)
	if cfg == nil {
		return "", nil
	}
	code, err := generateMonthly(db, cfg)
	if err != nil {
		err = fmt.Errorf("codegen %s: tăng bộ đếm config_code %d: %w", table, cfg.ID, err)
		if logger.S != nil {
			logger.S.Error(err)
		}
		return "", err
	}
	return code, nil
}

// findConfig — row config_code của bảng: (1) đúng chi nhánh → (2) toàn hệ thống (chi nhánh id nhỏ
// nhất). Bước 2 cũng là row "đếm chung" (option mã khách 5/6, dịch vụ seed 1 row) — ổn định vì FK
// config_code→branch là RESTRICT.
//
// Chọn row TẤT ĐỊNH: schema mới có UNIQUE (branch_id, parent_table) nên bước 1 tối đa 1 row; bước 2
// ưu tiên chi nhánh id nhỏ nhất rồi id nhỏ nhất — không bao giờ phụ thuộc thứ tự vật lý.
func findConfig(db *gorm.DB, table string, branchID *uint) *ConfigCode {
	baseQ := func() *gorm.DB {
		return db.Table("config_code cc").
			Select("cc.id, cc.branch_id, cc.parent_table, cc.infix_code, cc.serial_length, "+
				"cc.include_branch_code, cc.monthly_reset, COALESCE(br.code,'') as branch_code").
			Joins("LEFT JOIN branch br ON br.id = cc.branch_id").
			Where("cc.parent_table = ?", table)
	}

	if branchID != nil {
		var cfg ConfigCode
		q := baseQ().Where("cc.branch_id = ?", *branchID).Order("cc.id ASC")
		if q.Take(&cfg).Error == nil {
			return &cfg
		}
	}

	var cfg ConfigCode
	q := baseQ().Order("cc.branch_id ASC, cc.id ASC")
	if q.Take(&cfg).Error == nil {
		return &cfg
	}
	return nil
}

// generateMonthly tăng atomic bộ đếm (nextSerial) rồi ghép mã.
// Tương đương MyModel::generateMonthly(). Nếu cfg.MonthlyReset = false, serial dùng chung
// cố định year=0/month=0 (tăng vĩnh viễn, không reset theo tháng) và bỏ segment YYMM khỏi mã.
func generateMonthly(db *gorm.DB, cfg *ConfigCode) (string, error) {
	// Build prefix: BRAND.[BRANCH.]INFIX. — BRAND = mã thương hiệu (D22, `business_profile.code`).
	var parts []string
	// ProfileStrict: lỗi DB ⇒ trả lỗi (caller rollback), KHÔNG sinh mã thiếu tiền tố thương hiệu.
	prof, err := brand.ProfileStrict(db)
	if err != nil {
		return "", fmt.Errorf("đọc mã thương hiệu: %w", err)
	}
	if prof.Code != "" {
		parts = append(parts, prof.Code)
	}
	if cfg.IncludeBranchCode && cfg.BranchCode != "" {
		parts = append(parts, cfg.BranchCode)
	}
	if cfg.InfixCode != "" {
		parts = append(parts, strings.TrimRight(cfg.InfixCode, "."))
	}
	serial, ym, err := nextSerial(db, cfg.ID, cfg.MonthlyReset)
	if err != nil {
		return "", err
	}
	return joinCode(parts, ym, serial, cfg.SerialLength), nil
}

// nextSerial tăng atomic bộ đếm `config_code_monthly` của 1 row config_code. monthly = true ⇒ bộ
// đếm theo (năm, tháng) hiện tại theo múi giờ thương hiệu + trả kèm "YYMM"; false ⇒ bộ đếm cố định year=0/month=0.
// Lỗi ⇒ trả err, KHÔNG bịa số (xem GenerateErr).
func nextSerial(db *gorm.DB, configCodeID uint, monthly bool) (serial int64, ym string, err error) {
	year, month, ym := serialPeriod(db, monthly)

	// UPDATE/INSERT + LAST_INSERT_ID PHẢI chạy trên CÙNG 1 kết nối (LAST_INSERT_ID theo từng kết nối).
	// db không phải transaction thì 2 lệnh GORM rời có thể lấy 2 kết nối khác nhau trong pool ⇒ đọc
	// nhầm số ⇒ bọc Transaction: đang trong tx thì là SAVEPOINT trên cùng kết nối, không thì giữ 1
	// kết nối riêng cho cả 2 lệnh.
	err = db.Transaction(func(tx *gorm.DB) error {
		var bumpErr error
		serial, bumpErr = bumpSerial(func(query string, args ...any) (int64, error) {
			res := tx.Exec(query, args...)
			return res.RowsAffected, res.Error
		}, func() (id int64, err error) {
			err = tx.Raw("SELECT LAST_INSERT_ID()").Scan(&id).Error
			return id, err
		}, configCodeID, year, month)
		return bumpErr
	})
	if err != nil {
		return 0, "", err
	}
	if serial <= 0 {
		return 0, "", fmt.Errorf("bộ đếm trả số không hợp lệ %d", serial)
	}
	return serial, ym, nil
}

// bumpSerial tăng bộ đếm (configCodeID, year, month) và trả số mới. exec + lastInsertID PHẢI chạy trên
// CÙNG 1 kết nối (xem nextSerial).
//
// UPDATE trước, chỉ INSERT khi bộ đếm chưa có (số đầu tiên của kỳ). KHÔNG dùng thẳng
// INSERT ... ON DUPLICATE KEY UPDATE cho dòng đã có: InnoDB thử chèn id auto-increment mới trước khi
// phát hiện trùng khoá ⇒ giữ khoá X trên supremum của PRIMARY tới hết transaction ⇒ chặn MỌI lượt
// tăng bộ đếm khác trên bảng, kể cả bộ đếm của bảng khác (đo trên MySQL 9.7: tx vừa sinh mã đơn hàng
// chặn kết nối riêng sinh mã khách — nextCustomerSerial — tới lock-wait-timeout 50s, request tạo đơn
// mất 51s). UPDATE theo UNIQUE khớp đúng 1 dòng chỉ khoá dòng đó.
//
// Khe còn lại (đã đo, chấp nhận): lượt ĐẦU kỳ, 2 kết nối cùng chèn ⇒ bên thua trúng trùng ở nhánh INSERT
// và vẫn giữ supremum tới hết tx, chặn các lượt chèn dòng-đầu-kỳ KHÁC (không chặn UPDATE). Đổi sang
// INSERT IGNORE không giúp gì — trúng trùng cũng giữ supremum y hệt.
func bumpSerial(exec func(query string, args ...any) (int64, error), lastInsertID func() (int64, error),
	configCodeID uint, year, month int) (int64, error) {
	n, err := exec(serialUpdateSQL, configCodeID, year, month)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		// Chưa có bộ đếm của kỳ. ON DUPLICATE KEY đỡ trường hợp kết nối khác vừa tạo dòng này.
		if n, err = exec(serialUpsertSQL, configCodeID, year, month); err != nil {
			return 0, err
		}
		if n == 1 {
			return 1, nil // INSERT mới → serial = 1 (match PHP: affected=1 → return 1)
		}
	}
	// UPDATE ⇒ LAST_INSERT_ID chứa last_number + 1
	return lastInsertID()
}

const serialUpdateSQL = `UPDATE config_code_monthly SET last_number = LAST_INSERT_ID(last_number + 1)
	WHERE config_code_id = ? AND year = ? AND month = ?`

const serialUpsertSQL = `INSERT INTO config_code_monthly (config_code_id, year, month, last_number)
	VALUES (?, ?, ?, 1)
	ON DUPLICATE KEY UPDATE last_number = LAST_INSERT_ID(last_number + 1)`

// serialPeriod — khoá (năm, tháng) của bộ đếm + chuỗi "YYMM" theo múi giờ thương hiệu
// (brand.Location). monthly = false ⇒ (0, 0, "") = bộ đếm vĩnh viễn.
func serialPeriod(db *gorm.DB, monthly bool) (year, month int, ym string) {
	if !monthly {
		return 0, 0, ""
	}
	now := time.Now().In(brand.Location(db))
	return now.Year(), int(now.Month()), now.Format("0601") // YYMM
}

// joinCode ghép `parts` + [YYMM] + serial bằng dấu chấm, viết HOA. Serial width — auto-expand
// (match PHP: max(minWidth, strlen(serial))), tối thiểu 2.
func joinCode(parts []string, ym string, serial int64, minWidth int) string {
	if minWidth < 2 {
		minWidth = 2
	}
	width := minWidth
	if n := len(fmt.Sprintf("%d", serial)); n > width {
		width = n
	}
	out := make([]string, 0, len(parts)+2)
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	if ym != "" {
		out = append(out, ym)
	}
	out = append(out, fmt.Sprintf("%0*d", width, serial))
	return strings.ToUpper(strings.Join(out, "."))
}
