package dao

import (
	"encoding/json"

	"erpCluvix/pkg/seclog"

	"gorm.io/gorm"
)

type Dao struct{ DB *gorm.DB }

// InsertBatch ghi 1 batch Entry từ seclog sink. Best-effort: 1 lỗi không kéo
// theo cả app (chỉ mất audit rows, đã có zap log làm nguồn dự phòng).
func (d *Dao) InsertBatch(entries []seclog.Entry) {
	if len(entries) == 0 {
		return
	}
	rows := make([]SecurityAuditLog, 0, len(entries))
	for _, e := range entries {
		row := SecurityAuditLog{
			Event:     e.Event,
			UserID:    e.UserID,
			IP:        e.IP,
			RequestID: e.RequestID,
			Username:  e.Username,
			CreatedAt: e.CreatedAt,
		}
		if len(e.Detail) > 0 {
			if b, err := json.Marshal(e.Detail); err == nil {
				row.Detail = b
			}
		}
		rows = append(rows, row)
	}
	// CreateInBatches: 1 INSERT nhiều VALUES, tránh N round-trip.
	d.DB.CreateInBatches(rows, 100)
}

// AuditFilter — tham số lọc cho query nhật ký.
type AuditFilter struct {
	UserID uint
	Event  string   // 1 event cụ thể
	Events []string // nhóm event (vd login history) — ưu tiên hơn Event nếu set
	From   int64
	To     int64
}

// Query dựng câu truy vấn (chưa chạy) — caller paginate.
//
// Nhật ký bảo mật là toàn DB (1 doanh nghiệp/DB), cửa đọc duy nhất là super-user (handler gate).
func (d *Dao) Query(f AuditFilter) *gorm.DB {
	q := d.DB.Table("security_audit_log").
		Select("id, event, user_id, ip, request_id, username, detail, created_at")
	if f.UserID > 0 {
		q = q.Where("user_id = ?", f.UserID)
	}
	if len(f.Events) > 0 {
		q = q.Where("event IN ?", f.Events)
	} else if f.Event != "" {
		q = q.Where("event = ?", f.Event)
	}
	if f.From > 0 {
		q = q.Where("created_at >= ?", f.From)
	}
	if f.To > 0 {
		q = q.Where("created_at <= ?", f.To)
	}
	// `created_at` KHONG unique (nhieu su kien audit cung giay) va query nay di qua
	// pagination.Paginate (LIMIT/OFFSET) => thieu tiebreaker la 1 dong ra 2 lan o trang nay
	// va bien mat khoi trang kia. Don bang, khong JOIN nen `id` khong the ambiguous.
	return q.Order("created_at DESC, id DESC")
}

// DeleteStale xoá audit cũ hơn `before` (Unix) — retention. Trả số row đã xoá.
func (d *Dao) DeleteStale(before int64) (int64, error) {
	res := d.DB.Where("created_at < ?", before).Delete(&SecurityAuditLog{})
	return res.RowsAffected, res.Error
}
