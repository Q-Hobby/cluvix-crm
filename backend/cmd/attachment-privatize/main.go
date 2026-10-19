// Command attachment-privatize — chạy 1 LẦN sau khi deploy kho private (2026-10-19): chuyển file
// đính kèm khách cũ (attachment.file_url là URL tuyệt đối public-read) sang kho private, lưu KEY.
//
// Mặc định DRY-RUN (chỉ liệt kê). Chạy thật: --apply (hỏi gõ lại tên DB, trừ khi --yes).
// Xoá object public cũ: thêm --delete-public (chỉ xoá SAU khi DB đã đổi). Logic:
// internal/tools/attachmentprivatize. Hướng dẫn: DEPLOY.md mục "Chuyển file đính kèm cũ sang kho private".
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"erpCluvix/config"
	"erpCluvix/internal/tools/attachmentprivatize"
	"erpCluvix/pkg/optlock"
	"erpCluvix/pkg/storage"

	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type hostList []string

func (h *hostList) String() string { return strings.Join(*h, ",") }
func (h *hostList) Set(v string) error {
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			*h = append(*h, p)
		}
	}
	return nil
}

func main() {
	_ = godotenv.Load()

	dryRun := flag.Bool("dry-run", true, "chỉ liệt kê việc sẽ làm (mặc định); bị bỏ qua khi có --apply")
	apply := flag.Bool("apply", false, "chạy thật: ghi kho private + UPDATE attachment")
	deletePublic := flag.Bool("delete-public", false, "sau khi DB đổi thành công, xoá object public cũ (chỉ với --apply)")
	limit := flag.Int("limit", 0, "tối đa số bản ghi cần chuyển xử lý (0 = tất cả)")
	batch := flag.Int("batch", attachmentprivatize.DefaultBatch, "cỡ trang SELECT")
	dbName := flag.String("db", "", "tên database (mặc định DB_NAME trong .env)")
	yes := flag.Bool("yes", false, "bỏ bước gõ xác nhận tên DB khi --apply / --delete-public-from")
	manifestPath := flag.String("manifest", "", "file CSV ghi id,old_url,new_key của bản ghi đã chuyển (mặc định attachment-privatize-<db>-<thời điểm>.csv khi --apply)")
	deleteFrom := flag.String("delete-public-from", "", "chế độ dọn: xoá object public cũ theo manifest của lần --apply trước (chỉ khi bản ghi vẫn trỏ new_key)")
	var allow hostList
	flag.Var(&allow, "allow-host", "host được phép tải qua HTTP ngoài S3_CDN/S3_ENDPOINT/STORAGE_PUBLIC_BASE (lặp lại hoặc phân cách phẩy; 'host' = mọi port, 'host:port' = đúng port)")
	flag.Parse()
	_ = dryRun // dry-run là mặc định; chỉ --apply mới ghi.

	if *deletePublic && !*apply {
		log.Fatal("--delete-public chỉ dùng cùng --apply")
	}
	if *deleteFrom != "" && *apply {
		log.Fatal("--delete-public-from là chế độ riêng, không dùng cùng --apply")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if *dbName != "" {
		cfg.DBName = *dbName
	}
	if cfg.DBName == "" {
		log.Fatal("thiếu tên DB: đặt DB_NAME trong .env hoặc --db")
	}

	driver, _, err := storage.ResolveDriver(cfg.StorageDriver, cfg.S3Key)
	if err != nil {
		log.Fatalf("storage: %v", err)
	}
	pub, priv, err := storage.New(storage.Config{
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
	})
	if err != nil {
		log.Fatalf("storage: %v", err)
	}

	allowHosts := append(attachmentprivatize.HostsFromConfig(cfg.S3CDN, cfg.S3Endpoint, cfg.StoragePublicBase), allow...)

	mode := "DRY-RUN (không ghi gì)"
	if *deleteFrom != "" {
		mode = "XOÁ object public cũ theo manifest " + *deleteFrom
	} else if *apply {
		mode = "APPLY (ghi kho private + UPDATE DB)"
		if *deletePublic {
			mode += " + XOÁ object public cũ"
		}
	}
	fmt.Println("== attachment-privatize ==")
	fmt.Printf("DB đích       : %s @ %s:%s\n", cfg.DBName, cfg.DBHost, cfg.DBPort)
	fmt.Printf("Driver        : %s\n", driver)
	fmt.Printf("Kho private   : %s\n", describePrivate(driver, cfg))
	fmt.Printf("Kho public cũ : gốc URL %q\n", pub.URL(""))
	fmt.Printf("Host HTTP cho phép: %s\n", strings.Join(allowHosts, ", "))
	fmt.Printf("Chế độ        : %s\n", mode)

	if (*apply || *deleteFrom != "") && !*yes {
		fmt.Printf("Gõ lại đúng tên DB (%s) để chạy thật: ", cfg.DBName)
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		if strings.TrimSpace(line) != cfg.DBName {
			log.Fatal("tên DB không khớp — huỷ, không ghi gì")
		}
	}

	db, err := gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	if err != nil {
		log.Fatalf("kết nối DB: %v", err)
	}
	if err := optlock.Register(db); err != nil {
		log.Fatalf("optlock: %v", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		defer sqlDB.Close()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	m := &attachmentprivatize.Migrator{
		DB: db, Public: pub, Private: priv,
		Opts: attachmentprivatize.Options{
			Apply: *apply, DeletePublic: *deletePublic, Limit: *limit, Batch: *batch,
			AllowHosts: allowHosts, Log: os.Stdout,
		},
	}

	if *deleteFrom != "" {
		f, err := os.Open(*deleteFrom)
		if err != nil {
			log.Fatalf("mở manifest: %v", err)
		}
		rep, runErr := m.DeleteFromManifest(ctx, f)
		_ = f.Close()
		fmt.Println("== Báo cáo ==")
		fmt.Printf("Dòng manifest        : %d\n", rep.Candidates)
		fmt.Printf("Object public đã xoá : %d (giữ lại: %d)\n", rep.PublicDeleted, rep.PublicKept)
		finish(rep, runErr)
		return
	}

	if *apply {
		if *manifestPath == "" {
			*manifestPath = fmt.Sprintf("attachment-privatize-%s-%s.csv", cfg.DBName, time.Now().Format("20060102150405"))
		}
		f, err := os.OpenFile(*manifestPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			log.Fatalf("mở manifest: %v", err)
		}
		defer f.Close()
		m.Opts.Manifest = f
		fmt.Printf("Manifest      : %s (giữ file này để chạy --delete-public-from sau khi kiểm)\n", *manifestPath)
	}
	rep, runErr := m.Run(ctx)

	fmt.Println("== Báo cáo ==")
	fmt.Printf("Tổng attachment      : %d\n", rep.Total)
	fmt.Printf("Bỏ qua (đã là key)   : %d\n", rep.AlreadyKey)
	fmt.Printf("URL cũ đã xét        : %d\n", rep.Candidates)
	if *apply {
		fmt.Printf("Đã chuyển            : %d\n", rep.Migrated)
		if *deletePublic {
			fmt.Printf("Object public đã xoá : %d (giữ lại: %d)\n", rep.PublicDeleted, rep.PublicKept)
		}
	} else {
		fmt.Printf("Sẽ chuyển (dry-run)  : %d\n", rep.Planned)
	}
	finish(rep, runErr)
}

func finish(rep attachmentprivatize.Report, runErr error) {
	fmt.Printf("Lỗi                  : %d\n", len(rep.Failed))
	for _, f := range rep.Failed {
		fmt.Printf("  - id=%d: %s\n", f.ID, f.Reason)
	}
	if runErr != nil {
		log.Fatalf("dừng giữa chừng: %v (chạy lại an toàn — bản ghi đã chuyển sẽ bị bỏ qua)", runErr)
	}
	if len(rep.Failed) > 0 {
		os.Exit(2)
	}
}

func describePrivate(driver string, cfg *config.Config) string {
	switch driver {
	case storage.DriverS3, storage.DriverR2:
		b := cfg.S3PrivateBucket
		if strings.TrimSpace(b) == "" {
			b = cfg.S3Bucket
		}
		return fmt.Sprintf("bucket %q (folder <S3_FOLDER>/private)", b)
	default:
		return fmt.Sprintf("thư mục %q", cfg.StoragePrivatePath)
	}
}
