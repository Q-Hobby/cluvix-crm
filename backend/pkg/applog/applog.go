// Package applog — ghi log chẩn đoán CÓ CẤU TRÚC xuống bảng `app_event_log` để đọc bằng SQL.
//
// VÌ SAO CẦN: log hiện chỉ nằm trong file trên server (zap → tmp/run.log). Muốn biết "vì sao không lấy
// được tên khách Zalo" thì phải SSH vào máy grep — trong khi câu hỏi thật lại rất SQL: "3 ngày qua
// company nào, mã lỗi nào, bao nhiêu lần". Bảng này trả lời được bằng một câu GROUP BY.
//
// KHÔNG THAY THẾ zap. Quy tắc phân vai:
//   - zap  = dòng chảy vận hành, dày đặc, ngắn hạn.
//   - applog = sự kiện ĐÁNG TRUY VẤN LẠI, thưa, có ngữ cảnh (tenant, đối tượng, mã lỗi, JSON tuỳ ý).
//
// BA LUẬT BẤT DI BẤT DỊCH:
//  1. Ghi log KHÔNG được làm hỏng nghiệp vụ. Mọi lỗi ở đây bị nuốt (và chỉ báo qua zap).
//  2. Ghi log KHÔNG được làm CHẬM nghiệp vụ. Gọi Write là đẩy vào hàng đợi rồi trả về ngay; hàng đợi
//     đầy thì DROP chứ không chờ. Thà mất dòng log còn hơn giữ chân một request đang phục vụ khách.
//  3. KHÔNG ghi token/secret/mật khẩu vào `data`. Bảng này người vận hành đọc được và nằm trong mọi
//     bản dump DB.
package applog

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"erpCluvix/pkg/logger"
)

// Level — dùng chuỗi (không phải số) để câu SQL đọc được ngay mà không cần bảng tra.
const (
	LevelDebug = "debug"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
)

// Entry — 1 dòng log. Chỉ Source/Event là bắt buộc trên thực tế; phần còn lại tuỳ ngữ cảnh.
type Entry struct {
	Level   string
	Source  string // namespace chấm: "omnichat.zalo.profile"
	Event   string // mã ngắn ỔN ĐỊNH: "token_invalid" — đừng đổi chữ sau khi đã dùng
	Message string // câu tiếng Việt cho người đọc
	// story-40 (D15): bỏ field công ty — cột tương ứng của app_event_log cho phép NULL.
	BranchID  *uint
	UserID    *uint
	RefType   string // "omi_conversation", "omni_channel_account"...
	RefID     *uint64
	RequestID string
	ErrorCode string         // mã lỗi provider, vd "-216"
	Data      map[string]any // ngữ cảnh tuỳ ý — KHÔNG chứa secret
}

// row — bản ghi khớp cột bảng. Tách khỏi Entry để Entry giữ được kiểu thân thiện (map, *uint).
type row struct {
	ID        uint64          `gorm:"primaryKey"`
	CreatedAt int64           `gorm:"column:created_at"`
	Level     string          `gorm:"column:level"`
	Source    string          `gorm:"column:source"`
	Event     string          `gorm:"column:event"`
	Message   string          `gorm:"column:message"`
	BranchID  *uint           `gorm:"column:branch_id"`
	UserID    *uint           `gorm:"column:user_id"`
	RefType   *string         `gorm:"column:ref_type"`
	RefID     *uint64         `gorm:"column:ref_id"`
	RequestID *string         `gorm:"column:request_id"`
	ErrorCode *string         `gorm:"column:error_code"`
	Data      json.RawMessage `gorm:"column:data"`
}

func (row) TableName() string { return "app_event_log" }

// queueSize — hàng đợi cố ý NHỎ. Đây là log chẩn đoán thưa, không phải đường dẫn dữ liệu; hàng đợi to
// chỉ để tích lỗi lâu hơn trước khi vỡ, và ăn RAM khi DB có sự cố.
const queueSize = 512

// maxMessageRunes / maxDataBytes — cắt tại nguồn cho khớp cột. Không cắt thì MySQL từ chối cả dòng và
// ta mất đúng dòng log đang cần nhất.
//
// ⚠ ĐƠN VỊ LÀ **KÝ TỰ**, KHÔNG PHẢI BYTE: `VARCHAR(500)` của MySQL đếm 500 ký tự (utf8mb4), nên cắt
// theo byte vừa chặt quá 3 lần với tiếng Việt, vừa gây lỗi thật ở `truncate` (xem hàm đó).
const (
	maxMessageRunes = 500
	maxDataBytes    = 60 << 10 // 60KB
)

var (
	mu      sync.RWMutex
	queue   chan row
	started bool
	// done — worker đóng kênh này khi đã xả hết hàng đợi, để Close() biết lúc nào an toàn đóng DB.
	done chan struct{}

	// dropped — đếm số dòng bị bỏ vì hàng đợi đầy. Có con số này mới dám tin bảng log là đầy đủ;
	// không có nó thì log thiếu mà không ai biết là đang thiếu.
	dropped atomic.Int64
)

// Init gắn DB và bật worker. Gọi 1 lần lúc bootstrap. Chưa Init → mọi lời gọi Write là no-op (test,
// CLI, giai đoạn boot sớm đều chạy bình thường mà không cần biết applog tồn tại).
func Init(gdb *gorm.DB) {
	mu.Lock()
	defer mu.Unlock()
	if started || gdb == nil {
		return
	}
	queue = make(chan row, queueSize)
	done = make(chan struct{})
	started = true
	go worker(queue, done, gdb)
}

// Write đẩy 1 entry vào hàng đợi. KHÔNG BAO GIỜ chặn, không bao giờ trả lỗi.
func Write(e Entry) {
	// Gửi NGAY TRONG RLock (không phải lấy channel ra rồi thả khoá): Close() cần khoá ghi, nên khi nó
	// vào được là chắc chắn không còn ai đang gửi. Không có ràng buộc này thì có cửa sổ giữa RUnlock và
	// lệnh gửi để Close() đóng channel ⇒ "send on closed channel" ⇒ PANIC ngay giữa nghiệp vụ, tức là
	// vi phạm chính luật số 1 của package này. Gửi ở đây không chặn (có nhánh default) nên giữ khoá
	// trong lúc gửi là rẻ.
	mu.RLock()
	defer mu.RUnlock()
	if !started {
		return
	}

	select {
	case queue <- toRow(e):
	default:
		// Hàng đợi đầy = DB đang chậm/chết. Bỏ dòng này, đếm lại, và báo qua zap (chỗ duy nhất còn
		// đáng tin lúc đó). Tuyệt đối không chờ.
		if n := dropped.Add(1); n == 1 || n%100 == 0 {
			if logger.L != nil {
				logger.L.Warn("applog: hàng đợi đầy, bỏ log", zap.Int64("da_bo", n))
			}
		}
	}
}

// Close xả nốt hàng đợi rồi dừng worker. PHẢI gọi TRƯỚC db.Close() lúc shutdown: không có nó thì mỗi
// lần deploy/restart là mất sạch phần log còn trong hàng đợi, và worker còn ghi vào một DB vừa bị đóng.
//
// Có timeout vì shutdown không được phép treo: DB chết thì thà mất vài dòng log còn hơn giữ tiến trình
// không thoát được.
func Close(timeout time.Duration) {
	mu.Lock()
	q, d, on := queue, done, started
	if on {
		// Sau dòng này không còn Write nào gửi được nữa: Write giữ RLock trong suốt lệnh gửi, mà ta
		// đang giữ khoá ghi ⇒ mọi Write hoặc đã xong, hoặc sẽ thấy started=false.
		started = false
		queue = nil
	}
	mu.Unlock()
	if !on {
		return
	}

	close(q) // an toàn: không còn người gửi (xem trên). Worker xả nốt phần đã nằm trong hàng đợi.
	select {
	case <-d:
	case <-time.After(timeout):
		if logger.L != nil {
			logger.L.Warn("applog: hết thời gian chờ xả hàng đợi khi tắt", zap.Duration("timeout", timeout))
		}
	}
}

// Dropped — số dòng đã bị bỏ (để endpoint/health đọc khi cần đánh giá độ đầy đủ của bảng log).
func Dropped() int64 { return dropped.Load() }

// Các hàm tiện dụng — đặt tên theo level để call site đọc ra ngay mức độ.
func Info(e Entry)  { e.Level = LevelInfo; Write(e) }
func Warn(e Entry)  { e.Level = LevelWarn; Write(e) }
func Error(e Entry) { e.Level = LevelError; Write(e) }

func toRow(e Entry) row {
	if e.Level == "" {
		e.Level = LevelInfo
	}
	r := row{
		CreatedAt: time.Now().Unix(),
		Level:     e.Level,
		Source:    e.Source,
		Event:     e.Event,
		Message:   truncate(e.Message, maxMessageRunes),
		BranchID:  e.BranchID,
		UserID:    e.UserID,
		RefID:     e.RefID,
	}
	r.RefType = nilIfEmpty(e.RefType)
	r.RequestID = nilIfEmpty(e.RequestID)
	r.ErrorCode = nilIfEmpty(e.ErrorCode)

	if len(e.Data) > 0 {
		if b, err := json.Marshal(e.Data); err == nil && len(b) <= maxDataBytes {
			r.Data = b
		} else if err == nil {
			// Quá to: giữ dấu vết thay vì ném cả dòng đi.
			r.Data = json.RawMessage(`{"_truncated":true}`)
		}
	}
	return r
}

// worker — 1 goroutine duy nhất ghi DB. Một người ghi thì không có chuyện N request cùng mở N
// connection chỉ để ghi log.
func worker(q <-chan row, finished chan<- struct{}, gdb *gorm.DB) {
	defer close(finished)
	for r := range q {
		writeRow(gdb, r)
	}
}

func writeRow(gdb *gorm.DB, r row) {
	// Timeout riêng: DB treo thì worker phải thoát ra được, không kẹt vĩnh viễn giữ đầy hàng đợi.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := gdb.WithContext(ctx).Create(&r).Error; err != nil {
		// CHỈ zap. Tuyệt đối không gọi lại applog ở đây — ghi log lỗi bằng chính thứ vừa lỗi là
		// đường thẳng tới vòng lặp vô hạn.
		if logger.L != nil {
			logger.L.Warn("applog: ghi app_event_log thất bại",
				zap.String("source", r.Source), zap.String("event", r.Event), zap.Error(err))
		}
	}
}

// truncate — cắt còn tối đa n KÝ TỰ, luôn dừng ở biên rune.
//
// BUG ĐÃ SỬA (2026-08-16): bản đầu cắt theo byte (`s[:n]`). Message tiếng Việt bị cắt giữa một rune
// 3 byte ⇒ chuỗi UTF-8 hỏng ⇒ MySQL utf8mb4 từ chối NGUYÊN dòng (lỗi 1366) ⇒ mất đúng dòng log dài
// nhất, tức log của sự cố phức tạp nhất. Cơ chế sinh ra để chống mất log lại là thứ làm mất log.
// Test cũ không bắt được vì vô tình dùng ký tự 2 byte với ngưỡng chẵn → cắt rơi đúng biên.
func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	count := 0
	for i := range s { // range trên string đi theo RUNE, i luôn là biên rune hợp lệ
		if count == n {
			return s[:i]
		}
		count++
	}
	return s
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// UintPtr / Uint64Ptr — tiện ích cho call site khỏi phải khai biến tạm chỉ để lấy địa chỉ.
func UintPtr(v uint) *uint       { return &v }
func Uint64Ptr(v uint64) *uint64 { return &v }
