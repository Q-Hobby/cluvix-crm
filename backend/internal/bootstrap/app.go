package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"erpCluvix/config"
	"erpCluvix/db"
	"erpCluvix/internal/deps"
	"erpCluvix/internal/middleware"
	auditDaoPkg "erpCluvix/internal/modules/webapp/auth/audit/dao"
	deviceDao "erpCluvix/internal/modules/webapp/auth/device/dao"
	rbacSeed "erpCluvix/internal/modules/webapp/auth/rbac/seed"
	authSvc "erpCluvix/internal/modules/webapp/auth/rbac/service"
	exportHandlers "erpCluvix/internal/modules/webapp/export"
	hrSvc "erpCluvix/internal/modules/webapp/hr/user/service"
	"erpCluvix/internal/router"
	"erpCluvix/pkg/applog"
	"erpCluvix/pkg/cache"
	"erpCluvix/pkg/define"
	"erpCluvix/pkg/dexie"
	pkgExport "erpCluvix/pkg/export"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/logger"
	"erpCluvix/pkg/seclog"
	"erpCluvix/pkg/storage"
	"erpCluvix/pkg/token"
	"erpCluvix/pkg/validation"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// App holds the wired-up HTTP server and background job handles.
type App struct {
	srv      *http.Server
	bgCancel context.CancelFunc
}

// New wires up the full application: DB, Redis, services, router, background jobs.
// Caller must defer app.Close() after New returns successfully.
func New(cfg *config.Config) (*App, error) {
	if err := db.Connect(cfg.DSN(), cfg.IsProduction()); err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}

	// applog — log chẩn đoán xuống bảng `app_event_log` (đọc bằng SQL thay vì SSH grep file log).
	// Bật NGAY sau khi có DB để các bước boot phía dưới cũng ghi được. Best-effort: chưa Init thì mọi
	// applog.Write là no-op, không tác dụng phụ.
	applog.Init(db.DB)

	// Parallel post-DB init: column maps (INFORMATION_SCHEMA).
	// All independent blocking-I/O on the same DB path.
	var bootWG sync.WaitGroup
	bootWG.Add(2)
	go func() { defer bootWG.Done(); helpers.InitJSONColumnMap(db.DB) }()
	go func() { defer bootWG.Done(); helpers.InitDecimalColumnMap(db.DB) }()
	bootWG.Wait()

	authRedis := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort),
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDBAuth,
	})

	appCache := cache.New(
		fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort),
		cfg.RedisPassword,
		cfg.CachePrefix(),
		cfg.RedisDBCache,
	)

	tokenSvc := token.New(token.Config{
		Secret:            cfg.TokenSecret,
		AccessTTL:         cfg.TokenAccessTTL,
		RefreshTTL:        cfg.TokenRefreshTTL,
		MaxSessions:       cfg.TokenMaxSessions,
		IPBinding:         cfg.TokenIPBinding,
		Issuer:            cfg.TokenIssuer,
		Audience:          cfg.TokenAudience,
		AuthPrefix:        cfg.AuthPrefix(),
		CfgVer:            cfg.TokenCfgVer,
		DeviceBindingMode: cfg.DeviceBindingMode,
	}, authRedis)

	// Storage — 2 kho public/private theo STORAGE_DRIVER (local|s3|r2). Sai/thiếu cấu hình ⇒ DỪNG
	// khởi động (không rơi về local, không trả kho nil âm thầm) — xem pkg/storage/factory.go.
	publicStore, privateStore, err := storage.New(storage.Config{
		Driver:            cfg.StorageDriver,
		LocalPath:         cfg.StoragePath,
		LocalPrivatePath:  cfg.StoragePrivatePath,
		LocalPublicPrefix: cfg.StoragePublicPrefix,
		LocalPublicBase:   cfg.StoragePublicBase,
		S3Key:             cfg.S3Key,
		S3Secret:          cfg.S3Secret,
		S3Region:          cfg.S3Region,
		S3Bucket:          cfg.S3Bucket,
		S3PrivateBucket:   cfg.S3PrivateBucket,
		S3Endpoint:        cfg.S3Endpoint,
		S3CDN:             cfg.S3CDN,
		S3Folder:          cfg.S3Folder,
		S3PublicACL:       cfg.S3PublicACL,
		SigningKey:        cfg.StorageSigningKey,
		Prod:              cfg.IsProduction(),
	})
	if err != nil {
		return nil, fmt.Errorf("storage: %w", err)
	}

	// Globals set once at startup (whitelisted pattern — see CONVENTIONS.md §5).
	exportHandlers.DownloadURL = cfg.DownloadURL
	// Kho public mặc định cho đường ĐỌC ghép URL ảnh (storage.ResolveURL) ở mọi module.
	storage.SetDefaultPublic(publicStore)
	exportHandlers.SetDB(db.DB)
	dexie.SetCache(appCache)
	middleware.SetJWTVerifier(cfg.TokenSecret, cfg.TokenIssuer, cfg.TokenAudience)
	middleware.SetIPAllowlistMode(cfg.IPAllowlistMode)
	middleware.SetPermEnforce(cfg.PermEnforce)
	// Security audit sink: seclog.Event → buffered channel → batch INSERT (async).
	auditDao := &auditDaoPkg.Dao{DB: db.DB}
	seclog.InitSink(1000, auditDao.InsertBatch)
	// Register custom validator tags (phone_vn, ...) — idempotent via sync.Once.
	validation.RegisterCustomTags()

	rbacSvc := &authSvc.RbacService{DB: db.DB, Cache: appCache, TokenSvc: tokenSvc}

	// Startup seeder: đồng bộ catalog permission từ artifact codegen (thay Sync-from-FE thủ công).
	// Non-fatal mặc định (ưu tiên uptime); SEED_STRICT=true → fail-fast. Xem BMAD perm-codegen-seeder.
	seedPermissions(cfg, rbacSvc)

	userSvc := &hrSvc.UserService{DB: db.DB, Cache: appCache, TokenSvc: tokenSvc}

	r := router.Setup(cfg, deps.Deps{
		DB:           db.DB,
		Store:        publicStore,
		PrivateStore: privateStore,
		SignedURLTTL: cfg.StorageSignedURLTTL,
		Cache:        appCache,
		TokenSvc:     tokenSvc,
		RbacSvc:      rbacSvc,
		UserSvc:      userSvc,
		DownloadURL:  cfg.DownloadURL,
		Prod:         cfg.IsProduction(),
		TotpIssuer:   cfg.Code,
	})

	// Context cho các background job (helpers.RunBackground) — stops cleanly on shutdown signal.
	// (CRM standalone 2026-10-17: đã bỏ hẳn scheduler `cron_job`, thông báo/SSE, CLV, eventbus.)
	bgCtx, bgCancel := context.WithCancel(context.Background())

	// Background cron: dọn user_device không hoạt động (chống phình bảng), chạy mỗi 24h.
	helpers.RunBackground(bgCtx, "user_device_cleanup", func(ctx context.Context) {
		devDao := &deviceDao.Dao{DB: db.DB}
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		tick := func() {
			defer func() {
				if r := recover(); r != nil {
					logger.L.Error("user_device cleanup tick panic", zap.Any("panic", r))
				}
			}()
			cutoff := time.Now().AddDate(0, 0, -define.UserDeviceStaleDays).Unix()
			n, err := devDao.DeleteStale(cutoff)
			if err != nil {
				logger.L.Warn("user_device cleanup failed", zap.Error(err))
				return
			}
			if n > 0 {
				logger.L.Info("stale user_devices removed", zap.Int64("count", n))
			}
		}
		tick() // chạy 1 lần lúc boot, không đợi 24h
		for {
			select {
			case <-ctx.Done():
				logger.L.Info("user_device cleanup cron stopped")
				return
			case <-ticker.C:
				tick()
			}
		}
	})

	// Background cron: retention nhật ký bảo mật, chạy mỗi 24h.
	helpers.RunBackground(bgCtx, "security_audit_cleanup", func(ctx context.Context) {
		aDao := &auditDaoPkg.Dao{DB: db.DB}
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		tick := func() {
			defer func() {
				if r := recover(); r != nil {
					logger.L.Error("security_audit cleanup tick panic", zap.Any("panic", r))
				}
			}()
			cutoff := time.Now().AddDate(0, 0, -define.AuditRetentionDays).Unix()
			n, err := aDao.DeleteStale(cutoff)
			if err != nil {
				logger.L.Warn("security_audit cleanup failed", zap.Error(err))
				return
			}
			if n > 0 {
				logger.L.Info("stale security_audit_log removed", zap.Int64("count", n))
			}
		}
		tick()
		for {
			select {
			case <-ctx.Done():
				logger.L.Info("security_audit cleanup cron stopped")
				return
			case <-ticker.C:
				tick()
			}
		}
	})

	// Background: dọn file export cũ hơn EXPORT_FILE_TTL_HOURS (O-6). `cleanup.ScheduleFileDelete`
	// (30 phút) chỉ là hẹn giờ trong RAM — restart giữa chừng để lại file PII vĩnh viễn. Chạy 1 lần
	// lúc boot rồi mỗi giờ.
	helpers.RunBackground(bgCtx, "export_file_cleanup", func(ctx context.Context) {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		tick := func() {
			defer func() {
				if r := recover(); r != nil {
					logger.L.Error("export file cleanup tick panic", zap.Any("panic", r))
				}
			}()
			n, err := pkgExport.CleanupOld(pkgExport.DefaultDir, cfg.ExportFileTTL, time.Now())
			if err != nil {
				logger.L.Warn("export file cleanup failed", zap.Error(err))
			}
			if n > 0 {
				logger.L.Info("stale export files removed", zap.Int("count", n))
			}
		}
		tick()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				tick()
			}
		}
	})

	srv := &http.Server{
		Addr:    cfg.ListenAddr(),
		Handler: r,
		// ReadHeaderTimeout guards against slowloris. No ReadTimeout/WriteTimeout
		// so long-lived streaming responses are not cut mid-response (module `ai` — SSE
		// chat-customer-stream — đã xoá ở story-07; giữ cấu hình vì các response dài khác
		// (export, print) vẫn cần).
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	return &App{srv: srv, bgCancel: bgCancel}, nil
}

// seedPermissions đọc artifact embed → upsert/prune catalog rbac_auth_item lúc boot.
// Lỗi: SEED_STRICT=true → Fatal (chặn boot); mặc định → Errorf (logger đã có TG alert), app vẫn chạy.
func seedPermissions(cfg *config.Config, rbacSvc *authSvc.RbacService) {
	var items []authSvc.SyncPermInput
	if err := json.Unmarshal(rbacSeed.PermissionsJSON, &items); err != nil {
		if cfg.SeedStrict {
			logger.S.Fatalf("seed permissions unmarshal: %v", err)
		}
		logger.S.Errorf("seed permissions unmarshal FAILED (non-fatal): %v", err)
		return
	}
	res, err := rbacSvc.SeedPermissions(items, cfg.SeedPrune)
	if err != nil {
		if cfg.SeedStrict {
			logger.S.Fatalf("seed permissions: %v", err)
		}
		logger.S.Errorf("seed permissions FAILED (non-fatal): %v", err)
		return
	}
	logger.S.Infof("seed permissions: +%d ~%d -%d (nguồn: %d quyền)",
		len(res.Added), len(res.Updated), len(res.Pruned), len(items))
	// Mặc định KHÔNG prune (destructive). Có quyền thừa trong DB → cảnh báo, không xoá.
	if len(res.WouldPrune) > 0 {
		logger.S.Warnf("seed permissions: %d quyền trong DB không có ở artifact — KHÔNG xoá (SEED_PRUNE=false). Set SEED_PRUNE=true để prune có chủ đích: %v",
			len(res.WouldPrune), res.WouldPrune)
	}
}

// Serve starts the HTTP server and blocks until SIGINT/SIGTERM, then shuts down gracefully.
func (a *App) Serve(cfg *config.Config, startedAt time.Time) {
	logger.L.Info("ERP server starting",
		zap.String("addr", cfg.ListenAddr()),
		zap.String("env", cfg.Env),
		zap.Duration("startup", time.Since(startedAt)),
	)

	go func() {
		var err error
		if cfg.TLSCert != "" && cfg.TLSKey != "" {
			logger.L.Info("TLS enabled", zap.String("cert", cfg.TLSCert))
			err = a.srv.ListenAndServeTLS(cfg.TLSCert, cfg.TLSKey)
		} else {
			err = a.srv.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			logger.S.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.L.Info("Shutting down server...")

	a.bgCancel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := a.srv.Shutdown(ctx); err != nil {
		logger.L.Error("Server forced to shutdown", zap.Error(err))
	}
}

// Close releases DB pool. Call via defer after New.
func (a *App) Close() {
	// Xả nốt log đang trong hàng đợi TRƯỚC khi đóng DB — sau db.Close() thì worker chỉ ghi ra lỗi.
	// 3s là trần: shutdown không được treo vì một bảng log.
	applog.Close(3 * time.Second)
	db.Close()
}
