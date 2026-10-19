package main

import (
	"fmt"
	"time"

	"erpCluvix/config"
	"erpCluvix/internal/bootstrap"
	"erpCluvix/pkg/logger"

	"github.com/joho/godotenv"
	"github.com/shopspring/decimal"
)

func main() {
	startedAt := time.Now()

	// Cho phép decimal.Decimal marshal ra JSON number (không quote).
	// Tradeoff: số tiền > 2^53 (~9 x 10^15) sẽ mất precision ở JS. VND ERP hiếm khi vượt ngưỡng này.
	decimal.MarshalJSONWithoutQuotes = true

	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		panic(fmt.Sprintf("Failed to load config: %v", err))
	}

	logger.Init(logger.Config{
		Env:        cfg.Env,
		Production: cfg.IsProduction(),
		AppCode:    cfg.Code,
		DBName:     cfg.DBName,
		TGToken:    cfg.TGBotToken,
		TGChatID:   cfg.TGChatID,
	})
	defer logger.Sync()

	app, err := bootstrap.New(cfg)
	if err != nil {
		logger.S.Fatalf("bootstrap failed: %v", err)
	}
	defer app.Close()

	app.Serve(cfg, startedAt)
}
