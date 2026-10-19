package logger

import (
	"fmt"
	"os"
	"sync"
	"time"

	"erpCluvix/pkg/telegram"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	L *zap.Logger
	S *zap.SugaredLogger

	// tgClient — global Telegram client, exposed cho graceful shutdown (Sync).
	tgClient *telegram.Client
	initOnce sync.Once
)

type Config struct {
	Env string
	// Production — truyền config.IsProduction() (fail-closed: mọi APP_ENV không phải dev tường minh).
	// KHÔNG tự so Env == "prod" ở đây: APP_ENV=production/staging chạy chế độ prod (CORS, secret) mà
	// log vẫn ra dạng development (R-6, 2026-10-18).
	Production bool
	AppCode    string
	DBName     string // tên DB tenant — gắn vào nhãn alert, xem initInternal
	TGToken    string
	TGChatID   string
}

// Skip filter + dedup đã bỏ — user theo dõi triệt để mọi error.
// Rủi ro storm: 1 bug có thể fire hàng nghìn message/phút. Telegram rate limit
// (29 msg/s/group) sẽ kick in → worker pacing 50ms + 429 retry_after sleep tự throttle.
// Queue cap 256 → vượt sẽ drop với log stderr.

func Init(cfg Config) {
	initOnce.Do(func() {
		initInternal(cfg)
	})
}

func initInternal(cfg Config) {
	var zapCfg zap.Config
	if cfg.Production {
		zapCfg = zap.NewProductionConfig()
	} else {
		zapCfg = zap.NewDevelopmentConfig()
	}
	zapCfg.Level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)

	base, err := zapCfg.Build()
	if err != nil {
		// story-26 AC7: trước đây lỗi Build bị bỏ qua ⇒ `base` = nil ⇒ `logger.L` = nil ⇒ MỌI
		// `response.ServerError`/`logger.L.Error(...)` sau đó panic ngay từ request đầu tiên. Không
		// mở được sink log (permission/disk/config sai) là sự cố hạ tầng thật, nhưng crash giữa
		// request không an toàn hơn continue với logger no-op — fallback zap.NewNop() để service vẫn
		// phục vụ được, in lỗi ra stderr (không dùng được zap để tự log chính lỗi của zap).
		fmt.Fprintf(os.Stderr, "[logger] zapCfg.Build FAILED, fallback to no-op logger: %v\n", err)
		base = zap.NewNop()
	}

	if cfg.TGToken != "" && cfg.TGChatID != "" {
		// OnError CHỈ ghi stderr — KHÔNG dùng zap để tránh recursion
		// (Telegram fail → zap Error → trigger hook → gửi lại → fail → loop).
		tgClient = telegram.New(cfg.TGToken,
			telegram.WithOnError(func(err error) {
				fmt.Fprintf(os.Stderr, "[telegram] %s %v\n", time.Now().Format(time.RFC3339), err)
			}),
		)
		chatID := cfg.TGChatID
		// Nhãn có tên DB: DB-per-customer, mọi tenant bắn chung 1 kênh Telegram. APP_CODE do người
		// deploy tự đặt (có thể trùng/khó map ngược), còn DB_NAME là định danh chắc chắn của tenant.
		// Chỉ nằm ở nhãn, KHÔNG vào entry.Message ⇒ vân tay dedup (dedup.go) không đổi.
		label := cfg.AppCode + "|" + cfg.Env
		if cfg.DBName != "" {
			label += "|" + cfg.DBName
		}

		base = base.WithOptions(zap.Hooks(func(entry zapcore.Entry) error {
			if entry.Level < zapcore.ErrorLevel {
				return nil
			}
			msg := "🏷 [" + label + "]\n🔴 " + entry.Message
			if entry.Caller.Defined {
				msg += "\n📍 " + entry.Caller.String()
			}
			// FatalLevel/PanicLevel → zap sẽ os.Exit ngay sau hook return →
			// worker goroutine chưa kịp drain → message bị mất. Dùng sync để block.
			// KHÔNG dedup nhánh này: lỗi chí mạng chỉ xảy ra 1 lần rồi process chết.
			if entry.Level >= zapcore.FatalLevel {
				tgClient.SendToGroupSync(chatID, msg)
				return nil
			}
			// Dedup (pkg/logger/dedup.go) — tin đầu của mỗi loại lỗi vẫn gửi NGAY, chỉ nén
			// bản trùng trong cửa sổ và báo số lượng đã nén kèm tin kế tiếp.
			send, note := shouldSendAlert(entry.Message, time.Now())
			if !send {
				return nil
			}
			tgClient.SendToGroup(chatID, msg+note)
			return nil
		}))
	}

	L = base
	S = base.Sugar()
}

// Sync flushes zap buffer + best-effort drain Telegram queue.
// Gọi trong defer ở main để graceful shutdown không mất alert.
func Sync() {
	if L != nil {
		_ = L.Sync()
	}
	if tgClient != nil {
		// Sleep ngắn để worker drain queue. Không expose Flush API vì chỉ dùng ở shutdown.
		time.Sleep(500 * time.Millisecond)
	}
}
