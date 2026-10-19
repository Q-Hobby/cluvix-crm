// Package seclog — structured security events, format thống nhất để audit-log DB
// (Hướng 2) parse lại được từ log stream mà không phải đào từng call site.
//
// Quy ước: mọi event ghi level Info với msg "security" + field `sec_event`;
// KHÔNG log token/password/fingerprint đầy đủ — fingerprint rút gọn qua FP().
//
// Ngoài zap, Event còn đẩy 1 Entry vào DB sink (async, non-blocking) nếu đã
// InitSink — nền cho trang "Nhật ký bảo mật".
//
// Events: login_success, login_failed, login_no_fingerprint, new_device,
// refresh_fp_mismatch, access_ip_mismatch, device_evicted, device_revoked,
// revoked_token_reused, ip_not_allowed.
package seclog

import (
	"sync/atomic"
	"time"

	"erpCluvix/pkg/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// MaxEventLen — độ rộng cột `security_audit_log`.`event` (VARCHAR(64), migration
// 20260818133708). Event dài hơn bị CẮT chứ không được đẩy nguyên xuống DB.
//
// Lý do phải cắt ở đây chứ không "để DB báo lỗi": sink INSERT theo LÔ tới 64 dòng
// (xem InitSink). MySQL từ chối cả câu khi 1 giá trị quá dài (Error 1406) ⇒ 1 tên
// event dài làm mất trắng tới 63 dòng audit hợp lệ khác. Cắt 1 tên còn đọc được;
// mất cả lô thì sự cố thành vô hình. Đã xảy ra 2026-08-18.
const MaxEventLen = 64

// Entry là 1 dòng audit — khớp bảng security_audit_log.
type Entry struct {
	Event string
	// story-40 (D15): bỏ field công ty — cột tương ứng của security_audit_log cho phép NULL.
	UserID    *uint
	IP        string
	RequestID string
	Username  string
	Detail    map[string]any
	CreatedAt int64
}

var (
	sinkCh  chan Entry
	dropped int64
)

// InitSink khởi động DB sink: Event đẩy Entry vào buffered channel, 1 goroutine
// batch-flush. flush do caller (module audit) cấp — seclog không phụ thuộc DB/DAO.
// Non-blocking: channel đầy → drop + đếm (không bao giờ chặn request path).
func InitSink(bufSize int, flush func([]Entry)) {
	sinkCh = make(chan Entry, bufSize)
	// Goroutine đọc từ channel CỦA CHÍNH NÓ, không đọc lại biến toàn cục `sinkCh` mỗi vòng: nếu đọc
	// biến toàn cục thì sau một lần InitSink thứ hai, goroutine CŨ và MỚI cùng rút từ một channel —
	// entry rơi ngẫu nhiên vào flush func nào thắng. (Chỉ test gọi InitSink 2 lần, nhưng đây là loại
	// lỗi mà triệu chứng là "test lúc xanh lúc đỏ", tốn nhiều giờ hơn hẳn giá của 1 dòng này.)
	ch := sinkCh
	go func() {
		const batchMax = 64
		batch := make([]Entry, 0, batchMax)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		doFlush := func() {
			if len(batch) > 0 {
				flush(batch)
				batch = batch[:0]
			}
		}
		for {
			select {
			case e, ok := <-ch:
				if !ok {
					doFlush()
					return
				}
				batch = append(batch, e)
				if len(batch) >= batchMax {
					doFlush()
				}
			case <-ticker.C:
				doFlush()
			}
		}
	}()
}

// Dropped trả số Entry bị bỏ do channel đầy (metric giám sát sink).
func Dropped() int64 { return atomic.LoadInt64(&dropped) }

// Event ghi 1 security event ra zap + đẩy vào DB sink (nếu có). c có thể nil
// (gọi từ pkg không có request context). ip/request_id/user_id lấy từ
// context; flow chưa authenticated (login) truyền user_id/username
// tường minh qua seclog.UserID/Username. Story-40: bỏ company.
func Event(c *gin.Context, event string, fields ...zap.Field) {
	now := time.Now().Unix()
	if len(event) > MaxEventLen {
		event = event[:MaxEventLen]
	}
	entry := Entry{Event: event, CreatedAt: now, Detail: map[string]any{}}

	if c != nil {
		entry.IP = c.ClientIP()
		if v, ok := c.Get("request_id"); ok {
			if id, ok2 := v.(string); ok2 {
				entry.RequestID = id
			}
		}
		if v, ok := c.Get("userID"); ok {
			if uid, ok2 := v.(uint); ok2 && uid > 0 {
				entry.UserID = &uid
			}
		}
	}

	// Promote known keys sang cột; còn lại vào Detail JSON.
	for _, f := range fields {
		switch f.Key {
		case "user_id":
			u := uint(f.Integer)
			entry.UserID = &u
		case "username":
			entry.Username = f.String
		default:
			entry.Detail[f.Key] = fieldValue(f)
		}
	}

	// zap log (no-op khi logger.L nil — test/boot sớm).
	if logger.L != nil {
		base := append([]zap.Field{zap.String("sec_event", event)}, fields...)
		if entry.IP != "" {
			base = append(base, zap.String("ip", entry.IP))
		}
		if entry.RequestID != "" {
			base = append(base, zap.String("request_id", entry.RequestID))
		}
		logger.L.Info("security", base...)
	}

	// DB sink — non-blocking.
	if sinkCh != nil {
		select {
		case sinkCh <- entry:
		default:
			atomic.AddInt64(&dropped, 1)
		}
	}
}

// fieldValue rút giá trị Go từ zap.Field cho Detail JSON.
func fieldValue(f zap.Field) any {
	switch f.Type {
	case zapcore.StringType:
		return f.String
	case zapcore.BoolType:
		return f.Integer == 1
	case zapcore.Int64Type, zapcore.Int32Type, zapcore.Int16Type, zapcore.Int8Type:
		return f.Integer
	case zapcore.Uint64Type, zapcore.Uint32Type, zapcore.Uint16Type, zapcore.Uint8Type:
		return uint64(f.Integer)
	default:
		if f.Interface != nil {
			return f.Interface
		}
		if f.String != "" {
			return f.String
		}
		return f.Integer
	}
}

// UserID — dùng cho flow chưa authenticated (login), context không có userID.
func UserID(id uint) zap.Field { return zap.Uint("user_id", id) }

// FP rút gọn fingerprint còn 8 ký tự đầu — đủ correlate, không đủ replay.
func FP(fp string) zap.Field {
	if len(fp) > 8 {
		fp = fp[:8]
	}
	return zap.String("fp", fp)
}

// Username — cho event login_failed (chưa có userID).
func Username(u string) zap.Field { return zap.String("username", u) }
