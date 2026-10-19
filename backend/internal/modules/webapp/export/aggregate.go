package export

import (
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// cleanIDSubQuery clone idQuery, bỏ GROUP BY/HAVING (nếu có — thường tham chiếu alias chỉ
// tồn tại ở Select của query export gốc, ví dụ debtByUnitPrice ở treatment_session_debts) rồi
// chỉ Select 1 cột id — dùng an toàn làm subquery IN(...) mà không phụ thuộc Select/Group/Having
// của caller. Gorm merge Group/Having khi Session({}) clone nên phải xoá thủ công qua Clauses map.
func cleanIDSubQuery(idQuery *gorm.DB, idCol string) *gorm.DB {
	clone := idQuery.Session(&gorm.Session{})
	delete(clone.Statement.Clauses, "GROUP BY")
	return clone.Select(idCol)
}

// loadSumByFilteredIDs tính SUM(sumCol) trên table, GROUP BY groupCol — scope theo đúng
// tập id mà idQuery (cùng filter/join với query export chính) trả về, làm subquery IN.
// Chạy 1 lần cho cả export, thay vì correlated subquery lặp lại theo từng dòng.
// sumCol là cột tiền decimal(20,2) ⇒ giữ decimal.Decimal (KHÔNG float64); chỉ đổi sang số Excel ở ô ghi.
func loadSumByFilteredIDs(idQuery *gorm.DB, idCol, table, groupCol, sumCol string) map[int]decimal.Decimal {
	idSubQ := cleanIDSubQuery(idQuery, idCol)

	var rows []struct {
		GroupID int
		Total   decimal.Decimal
	}
	idQuery.Session(&gorm.Session{NewDB: true}).Table(table).
		Select(groupCol+" as group_id, COALESCE(SUM("+sumCol+"),0) as total").
		Where(groupCol+" IN (?)", idSubQ).
		Group(groupCol).
		Find(&rows)

	m := make(map[int]decimal.Decimal, len(rows))
	for _, r := range rows {
		m[r.GroupID] = r.Total
	}
	return m
}

// loadUserNamesByGroupID gộp tên user (qua userIDCol) theo groupCol trên table — scope theo
// idQuery, 1 lần cho cả export. Dùng cho quan hệ 1-nhiều như kỹ thuật viên/phụ tá của 1 điều trị.
func loadUserNamesByGroupID(idQuery *gorm.DB, idCol, table, groupCol, userIDCol string) map[int]string {
	idSubQ := cleanIDSubQuery(idQuery, idCol)

	var rows []struct {
		GroupID int
		Name    string
	}
	idQuery.Session(&gorm.Session{NewDB: true}).Table(table+" t").
		Select("t."+groupCol+" as group_id, GROUP_CONCAT(up.fullname SEPARATOR ', ') as name").
		Joins("LEFT JOIN user_profile up ON up.user_id = t."+userIDCol).
		Where("t."+groupCol+" IN (?)", idSubQ).
		Group("t." + groupCol).
		Find(&rows)

	m := make(map[int]string, len(rows))
	for _, r := range rows {
		m[r.GroupID] = r.Name
	}
	return m
}

// loadLatestAppointmentDateByCustomerID lấy appointment_date của lịch hẹn gần nhất (theo id)
// từng customer_id — scope theo idQuery, 1 lần cho cả export, thay vì correlated subquery
// (ORDER BY id DESC LIMIT 1) lặp lại theo từng dòng của query chính.
func loadLatestAppointmentDateByCustomerID(idQuery *gorm.DB, idCol string) map[int]int64 {
	idSubQ := cleanIDSubQuery(idQuery, idCol)

	var rows []struct {
		CustomerID      int
		AppointmentDate int64
	}
	idQuery.Session(&gorm.Session{NewDB: true}).Table("appointment a").
		Select("a.customer_id, a.appointment_date").
		Joins(`JOIN (
			SELECT customer_id, MAX(id) as max_id
			FROM appointment
			WHERE customer_id IN (?)
			GROUP BY customer_id
		) latest ON latest.customer_id = a.customer_id AND latest.max_id = a.id`, idSubQ).
		Find(&rows)

	m := make(map[int]int64, len(rows))
	for _, r := range rows {
		m[r.CustomerID] = r.AppointmentDate
	}
	return m
}
