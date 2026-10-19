package logger

import (
	"bytes"
	"runtime"
	"strconv"
	"sync"
)

// requestPaths — map goroutine_id → "METHOD /url" của request đang xử lý.
// Cho phép gorm.Trace (chạy đồng bộ trong goroutine request) lấy API path
// kèm vào SQL error log mà KHÔNG cần refactor toàn bộ handlers gọi WithContext.
//
// Hạn chế: nếu DB query chạy ở goroutine khác (cron, background task), gid khác →
// path không tìm thấy → log chỉ có "SQL Error: ..." như cũ. Đó là behavior chấp nhận.
var requestPaths sync.Map

// SetRequestPath gọi từ middleware đầu request.
func SetRequestPath(path string) {
	requestPaths.Store(goid(), path)
}

// ClearRequestPath gọi từ middleware (defer) cuối request — tránh leak map.
func ClearRequestPath() {
	requestPaths.Delete(goid())
}

// GetRequestPath lấy path của request đang chạy trong goroutine hiện tại.
// Trả "" nếu không có (vd background goroutine, cron).
func GetRequestPath() string {
	if v, ok := requestPaths.Load(goid()); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// goid extract goroutine ID bằng cách parse runtime.Stack.
// Chạy ~200ns, chỉ gọi khi có log Error nên không ảnh hưởng perf.
// Format: "goroutine 123 [running]:\n..."
func goid() uint64 {
	b := make([]byte, 64)
	n := runtime.Stack(b, false)
	line := b[:n]
	// Skip "goroutine "
	i := bytes.IndexByte(line, ' ')
	if i < 0 {
		return 0
	}
	rest := line[i+1:]
	j := bytes.IndexByte(rest, ' ')
	if j < 0 {
		return 0
	}
	id, _ := strconv.ParseUint(string(rest[:j]), 10, 64)
	return id
}
