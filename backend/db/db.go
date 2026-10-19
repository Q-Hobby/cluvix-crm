package db

import (
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"erpCluvix/pkg/logger"
	"erpCluvix/pkg/optlock"
)

var DB *gorm.DB

func Connect(dsn string, isProduction bool) error {
	var err error
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.NewGormLogger(),
	})
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	// Khoá lạc quan updated_at (D6): mọi UPDATE vào 9 bảng CRM bump GREATEST(now, updated_at+1).
	if err := optlock.Register(DB); err != nil {
		return fmt.Errorf("register optlock callback: %w", err)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("get sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(200)
	sqlDB.SetMaxIdleConns(20)
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)

	fmt.Println("Database connected successfully")
	return nil
}

func Close() {
	if DB != nil {
		sqlDB, _ := DB.DB()
		if sqlDB != nil {
			err := sqlDB.Close()
			if err != nil {
				return
			}
		}
	}
}
