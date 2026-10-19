package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	cmd := flag.String("cmd", "up", "up | down | create | version | force")
	name := flag.String("name", "", "migration name (for create)")
	dbName := flag.String("db", "", "database name (default: DB_NAME from .env)")
	steps := flag.Int("steps", 1, "number of steps (for down)")
	ver := flag.Int("force", -1, "force version")
	confirm := flag.Bool("confirm", false, "explicit confirm for destructive ops (down, force)")
	autoRecover := flag.Bool("auto-recover", false, "auto force-back to previous version if dirty state detected during up (destructive; requires migrations idempotent)")
	flag.Parse()

	if *dbName == "" {
		*dbName = env("DB_NAME", "")
	}

	switch *cmd {
	case "create":
		if *name == "" {
			log.Fatal("Usage: -cmd=create -name=add_column_to_table")
		}
		createMigration(*name)

	case "up", "down", "version", "force":
		if *dbName == "" {
			log.Fatal("Set DB_NAME in .env or use -db=<name>")
		}
		dsn := buildDSN(*dbName)
		m := openMigrate(dsn)
		defer m.Close()

		switch *cmd {
		case "up":
			oldVer, dirty, _ := m.Version()
			// Auto-recover: nếu dirty thì force version về previous-successful rồi retry.
			// Yêu cầu migration phải idempotent — re-run an toàn. Vì force là destructive
			// (ghi đè schema_migrations), bắt buộc explicit -auto-recover flag.
			if dirty {
				if !*autoRecover {
					log.Fatalf("migrate up: detected dirty state at version %d. Re-run với -auto-recover để force về previous-successful rồi retry (yêu cầu migrations idempotent), hoặc dùng -cmd=force -force=<version> -confirm để chọn version cụ thể.", oldVer)
				}
				// LƯU Ý: -auto-recover ĐÃ là flag opt-in tường minh (deploy chủ động truyền để tự phục hồi
				// dirty-state KHÔNG cần người). KHÔNG bắt thêm -confirm ở đây — sẽ phá pipeline deploy chạy nền
				// (regression 2026-07-18: deploy `migrate up -auto-recover` báo "Migrate thất bại"). Hàng rào
				// chống-chạm-DB-thật đúng chỗ là ở testhelpers (guard DB==DB_NAME) + seed idempotent, KHÔNG ở đây.
				prevVer := previousMigrationVersion(int(oldVer))
				log.Printf("migrate up: detected dirty state at version %d → auto-force về %d (auto-recover enabled)", oldVer, prevVer)
				if err := m.Force(prevVer); err != nil {
					log.Fatalf("migrate up: auto-force failed: %v", err)
				}
				oldVer = uint(prevVer)
			}
			if err := m.Up(); err != nil {
				if err == migrate.ErrNoChange {
					log.Printf("migrate up: không có migration mới (version %d)", oldVer)
				} else {
					log.Fatalf("migrate up: %v", err)
				}
			} else {
				newVer, _, _ := m.Version()
				log.Printf("migrate up: %d → %d thành công", oldVer, newVer)
			}
		case "down":
			if !*confirm {
				log.Fatalf("migrate down %d (DB=%s): destructive op. Re-run với -confirm để xác nhận.", *steps, *dbName)
			}
			if err := m.Steps(-*steps); err != nil {
				log.Fatalf("migrate down: %v", err)
			}
			log.Printf("migrate down %d: done", *steps)
		case "version":
			v, dirty, err := m.Version()
			if err != nil {
				log.Fatalf("version: %v", err)
			}
			log.Printf("version: %d (dirty: %v)", v, dirty)
		case "force":
			if *ver < 0 {
				log.Fatal("Usage: -cmd=force -db=<name> -force=<version> -confirm")
			}
			if !*confirm {
				log.Fatalf("force to version %d (DB=%s): destructive op (overrides DB schema state). Re-run với -confirm để xác nhận.", *ver, *dbName)
			}
			if err := m.Force(*ver); err != nil {
				log.Fatalf("force: %v", err)
			}
			log.Printf("forced to version %d", *ver)
		}

	default:
		log.Fatalf("Unknown command: %s", *cmd)
	}
}

func buildDSN(dbName string) string {
	pass := env("DB_PASS", "")
	if pass == "" {
		pass = env("DB_PASSWORD", "")
	}
	return fmt.Sprintf("mysql://%s:%s@tcp(%s:%s)/%s?multiStatements=true",
		env("DB_USER", "root"), url.QueryEscape(pass),
		env("DB_HOST", "localhost"), env("DB_PORT", "3306"),
		dbName)
}

func openMigrate(dsn string) *migrate.Migrate {
	m, err := migrate.New("file://migrations", dsn)
	if err != nil {
		log.Fatalf("open migrate: %v", err)
	}
	return m
}

func createMigration(name string) {
	ts := time.Now().Format("20060102150405")
	up := fmt.Sprintf("migrations/%s_%s.up.sql", ts, name)
	down := fmt.Sprintf("migrations/%s_%s.down.sql", ts, name)
	if err := os.WriteFile(up, []byte("-- migrate up\n"), 0o600); err != nil {
		log.Fatalf("write %s: %v", up, err)
	}
	if err := os.WriteFile(down, []byte("-- migrate down\n"), 0o600); err != nil {
		log.Fatalf("write %s: %v", down, err)
	}
	log.Printf("Created:\n  %s\n  %s", up, down)
}

// previousMigrationVersion: tìm version migration kế trước `current` từ thư mục migrations/.
// Dùng để auto-force về previous-successful khi DB đang dirty.
// Nếu không có previous (current là migration đầu) → trả 0.
func previousMigrationVersion(current int) int {
	files, err := filepath.Glob("migrations/*.up.sql")
	if err != nil || len(files) == 0 {
		return 0
	}
	var versions []int
	for _, f := range files {
		name := filepath.Base(f)
		prefix := strings.SplitN(name, "_", 2)[0]
		v, err := strconv.Atoi(prefix)
		if err != nil {
			continue
		}
		versions = append(versions, v)
	}
	sort.Ints(versions)
	prev := 0
	for _, v := range versions {
		if v >= current {
			break
		}
		prev = v
	}
	return prev
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
