// Package storage — lưu file upload sau một interface, chọn driver bằng STORAGE_DRIVER.
//
// Hai kho tách biệt (user chốt 2026-10-19):
//   - PUBLIC  — avatar, logo, ảnh dịch vụ/chi nhánh/nhóm dịch vụ, ảnh TinyMCE. URL(key) trả đường
//     dẫn trình duyệt tải thẳng được (CDN / static /uploads).
//   - PRIVATE — file đính kèm khách hàng (crm/attachment). URL(key) LUÔN rỗng: chỉ đọc qua backend
//     (có JWT + kiểm quyền) bằng Open rồi stream.
//
// DB lưu KEY (đường dẫn tương đối trong kho, không domain, không folder prefix của S3). URL được
// ghép lúc trả về qua PublicURL — đổi CDN/driver không phải sửa một dòng dữ liệu nào. Giá trị cũ đã
// là URL tuyệt đối (http/https) được trả nguyên (tương thích ngược, không migration dữ liệu).
package storage

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync/atomic"
)

var (
	// ErrNotFound — key không tồn tại trong kho. Open trả lỗi này thay vì lỗi I/O để caller trả
	// 404 thay vì 500 (file bị xoá tay khỏi kho là chuyện bình thường).
	ErrNotFound = errors.New("storage: not found")
	// ErrUnsafePath — key rỗng, tuyệt đối, hoặc chứa đoạn ".." (thoát khỏi thư mục gốc).
	ErrUnsafePath = errors.New("storage: unsafe path")
)

// ObjectInfo — metadata đọc kèm khi Open. Size < 0 = không biết.
type ObjectInfo struct {
	Size        int64
	ContentType string
}

// Store — nơi lưu file đã upload.
//
// Mọi method nhận context vì driver S3/R2 gọi MẠNG: không deadline nghĩa là storage treo thì
// goroutine treo vĩnh viễn. Driver tự gắn thêm trần thời gian, ctx của caller chỉ có thể làm NGẮN hơn.
type Store interface {
	// Save ghi nội dung r vào key. key do SERVER sinh (không lấy từ tên file client gửi).
	// contentType nên là MIME đã sniff bằng magic bytes, KHÔNG phải header client tự khai.
	Save(ctx context.Context, r io.Reader, key, contentType string) error
	// Open đọc lại file. Không tồn tại ⇒ ErrNotFound. Caller PHẢI Close.
	Open(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error)
	// Delete xoá file. Không tồn tại KHÔNG phải lỗi.
	Delete(ctx context.Context, key string) error
	// URL — đường dẫn công khai của key. Kho private LUÔN trả "" (không có đường công khai).
	URL(key string) string
	// Public — kho này có phục vụ công khai không.
	Public() bool
}

// SafeKey chặn key nguy hiểm ở MỌI driver: key do server sinh, nhưng kiểm ở tầng kho để một lỗi
// lập trình không thành lỗ hổng path traversal (local) hay ghi đè object ngoài prefix (S3).
func SafeKey(key string) error {
	if key == "" || strings.HasPrefix(key, "/") || strings.ContainsAny(key, "\\\x00") {
		return ErrUnsafePath
	}
	for _, seg := range strings.Split(key, "/") {
		if seg == ".." {
			return ErrUnsafePath
		}
	}
	return nil
}

// IsAbsoluteURL — giá trị đã là URL tuyệt đối (dữ liệu cũ trước 2026-10-19 lưu nguyên URL CDN).
func IsAbsoluteURL(v string) bool {
	l := strings.ToLower(v)
	return strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://") || strings.HasPrefix(l, "//")
}

// PublicURL ghép URL trả FE cho một giá trị lưu trong DB.
//
//   - rỗng ⇒ rỗng (không có ảnh).
//   - URL tuyệt đối (dữ liệu cũ) ⇒ trả nguyên — tương thích ngược, không cần migration.
//   - bắt đầu bằng "/" ⇒ đã là đường dẫn trên site, trả nguyên (không ghép prefix lần nữa).
//   - còn lại là KEY ⇒ s.URL(key). s nil (chưa cấu hình, test) ⇒ trả nguyên.
func PublicURL(s Store, v string) string {
	if v == "" || IsAbsoluteURL(v) || strings.HasPrefix(v, "/") || s == nil {
		return v
	}
	return s.URL(v)
}

// ToKey — chiều ngược của PublicURL: FE gửi lại chính URL nó nhận được (form sửa gửi nguyên
// model) ⇒ bóc gốc URL của kho để lưu lại KEY. URL không thuộc kho (CDN cũ, link ngoài) giữ nguyên.
//
// Bắt buộc gọi ở mọi chỗ ghi cột ảnh từ payload: không bóc thì DB lưu URL tuyệt đối, và
// DeleteOldIfChanged so "key cũ" với "URL mới" thấy khác nhau ⇒ XOÁ NHẦM ảnh đang dùng.
func ToKey(s Store, v string) string {
	if s == nil || v == "" {
		return v
	}
	base := s.URL("")
	if base != "" && strings.HasPrefix(v, base) {
		return strings.TrimPrefix(v, base)
	}
	return v
}

// ── Kho public mặc định (đặt 1 lần lúc boot) ──────────────────────────────
//
// Các handler ĐỌC trả ảnh nằm rải ở hàng chục module (khách hàng lồng trong lịch hẹn, phiếu thu,
// báo cáo...). Kéo Store qua Deps của từng module chỉ để ghép URL là nhân bản wiring vô ích, nên
// đường đọc dùng kho mặc định này (cùng khuôn global-set-once với dexie.SetCache). Đường GHI
// (upload/xoá) vẫn nhận Store tường minh qua Deps.

type storeHolder struct{ s Store }

var defaultPublic atomic.Pointer[storeHolder]

// SetDefaultPublic đặt kho public dùng cho ResolveURL. Gọi 1 lần ở bootstrap.
func SetDefaultPublic(s Store) { defaultPublic.Store(&storeHolder{s: s}) }

// DefaultPublic trả kho public mặc định (nil nếu chưa đặt).
func DefaultPublic() Store {
	if h := defaultPublic.Load(); h != nil {
		return h.s
	}
	return nil
}

// ResolveURL = PublicURL(DefaultPublic(), v).
func ResolveURL(v string) string { return PublicURL(DefaultPublic(), v) }

// ResolveFields ghép URL cho các cột ảnh của 1 row map (kết quả GORM scan vào map).
func ResolveFields(row map[string]interface{}, fields ...string) {
	if row == nil {
		return
	}
	for _, f := range fields {
		switch v := row[f].(type) {
		case string:
			row[f] = ResolveURL(v)
		case []byte:
			row[f] = ResolveURL(string(v))
		case *string:
			if v != nil {
				s := ResolveURL(*v)
				row[f] = &s
			}
		}
	}
}

// ResolveRows — ResolveFields cho cả slice.
func ResolveRows(rows []map[string]interface{}, fields ...string) {
	for _, r := range rows {
		ResolveFields(r, fields...)
	}
}

// ResolveNested ghép URL cho object LỒNG row[key] (vd `customer: {avatar}` do AttachRelation gắn).
// Nhận cả object đơn lẫn mảng object.
func ResolveNested(rows []map[string]interface{}, key string, fields ...string) {
	for _, r := range rows {
		if r == nil {
			continue
		}
		switch v := r[key].(type) {
		case map[string]interface{}:
			ResolveFields(v, fields...)
		case []map[string]interface{}:
			ResolveRows(v, fields...)
		case []interface{}:
			for _, e := range v {
				if m, ok := e.(map[string]interface{}); ok {
					ResolveFields(m, fields...)
				}
			}
		}
	}
}

// TableImageColumns — cột ảnh lưu KEY kho public, theo bảng. Nguồn DUY NHẤT để helper dùng chung
// (handlerutil.AttachRelation*) tự ghép URL cho object lồng mà không phải sửa từng call site.
// Thêm cột ảnh mới (upload qua imaging vào kho public) ⇒ thêm vào đây.
var TableImageColumns = map[string][]string{
	"customer":         {"avatar"},
	"user_profile":     {"avatar"},
	"business_profile": {"logo_image"},
}

// ResolveTableImages ghép URL cho các cột ảnh của `table` (theo TableImageColumns) có trong rows.
// Bảng không có cột ảnh ⇒ no-op.
func ResolveTableImages(rows []map[string]interface{}, table string) {
	if cols, ok := TableImageColumns[table]; ok {
		ResolveRows(rows, cols...)
	}
}
