package logger

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
	gormlogger "gorm.io/gorm/logger"
)

type GormLogger struct {
	SlowThreshold time.Duration
}

func NewGormLogger() *GormLogger {
	return &GormLogger{SlowThreshold: 200 * time.Millisecond}
}

func (g *GormLogger) LogMode(gormlogger.LogLevel) gormlogger.Interface { return g }

func (g *GormLogger) Info(_ context.Context, msg string, args ...interface{})  {}
func (g *GormLogger) Warn(_ context.Context, msg string, args ...interface{})  {}
func (g *GormLogger) Error(_ context.Context, msg string, args ...interface{}) {}

func (g *GormLogger) Trace(_ context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	sql, rows := fc()
	if strings.Contains(sql, "appointment") && strings.Contains(sql, "checkin_date") {
		L.Info(fmt.Sprintf("SQL [checkin_date debug]: %s | rows=%d | time=%s | err=%v", sql, rows, time.Since(begin), err))
	}
	if err == nil {
		// SlowThreshold trước đây được KHAI BÁO và set 200ms nhưng không dòng nào đọc tới — hàm này
		// return ngay khi err == nil, nên repo thực chất KHÔNG có lưới log query chậm. Hệ quả: một
		// query 3 giây hoàn toàn vô hình, và grep "SLOW SQL" ra rỗng bị hiểu nhầm thành "không có
		// query nào chậm" (đã dẫn tới một kết luận sai khi truy vấn đề import nhân sự 90 giây).
		//
		// stderr chứ KHÔNG L.Error: query chậm là số liệu hiệu năng cần nhìn thấy, không phải sự cố
		// đáng bắn Telegram — để L.Error thì mỗi lần dev chạy báo cáo nặng là spam cả kênh alert.
		if g.SlowThreshold > 0 {
			if elapsed := time.Since(begin); elapsed >= g.SlowThreshold {
				pathPart := ""
				if path := GetRequestPath(); path != "" {
					pathPart = " @ " + path
				}
				fmt.Fprintf(os.Stderr, "[SLOW SQL %v]%s rows=%d | %s\n",
					elapsed.Round(time.Millisecond), pathPart, rows, strings.Join(strings.Fields(sql), " "))
			}
		}
		return
	}
	errMsg := err.Error()
	// "record not found" là return chuẩn của gorm khi First/Take không có row — không phải bug.
	if strings.Contains(errMsg, "record not found") {
		return
	}
	// 1451 FK constraint on DELETE = user xóa data đang được tham chiếu — nghiệp vụ bình thường, không phải bug.
	// 1452 FK ref missing on INSERT/UPDATE — handler đã trả 422, không cần alert.
	if strings.Contains(errMsg, "1451") || strings.Contains(errMsg, "1452") {
		return
	}
	// context canceled = client (mobile/Zalo) tự ngắt kết nối trước khi query trả về (ScopedClient/ClientDB
	// chain c.Request.Context()) — không phải lỗi SQL, không cần alert.
	if errors.Is(err, context.Canceled) || strings.Contains(errMsg, "context canceled") {
		return
	}
	elapsed := time.Since(begin)
	// Inject API path nếu query chạy đồng bộ trong goroutine của request.
	pathPart := ""
	if path := GetRequestPath(); path != "" {
		pathPart = " @ " + path
	}
	L.Error(fmt.Sprintf("SQL Error%s: %s\nQuery: %s\nRows: %d | Time: %s",
		pathPart, errMsg, sql, rows, elapsed.String()),
		zap.Skip(),
	)
}
