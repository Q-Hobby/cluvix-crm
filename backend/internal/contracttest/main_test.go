//go:build integration

// Package contracttest — lưới kiểm chứng CONTRACT của response JSON, không phải test nghiệp vụ.
//
// Mục đích (epic decimal-json-number): khẳng định mọi field DECIMAL trong response API là
// **JSON number**, không phải JSON string. Cơ chế đúng đã có sẵn trong repo:
//
//	bootstrap/app.go        InitDecimalColumnMap(db)                    // map table→cột DECIMAL
//	pkg/helpers             ParseDecimalFieldsForTables(rows, tables…)  // []uint8 → decimal.Decimal
//	main.go:20              decimal.MarshalJSONWithoutQuotes = true     // decimal.Decimal → number
//
// Lỗ hổng: handler `.Take/.First/.Scan/.Find` vào `map[string]interface{}` nhận `[]uint8` từ
// driver; nếu KHÔNG gọi `ParseDecimalFieldsForTables` thì giá trị đi thẳng ra `encoding/json`
// dưới dạng []byte → base64/string, không bao giờ là number. `pagination.Paginate*` đã tự gọi
// helper nên endpoint list an toàn; query map thủ công thì không.
//
// # Cách chạy
//
//	TEST_MYSQL_DSN='root:***@tcp(localhost:3306)/cluvix_decimal_test?charset=utf8mb4&parseTime=True&loc=Local&multiStatements=true' \
//	  go test -tags=integration ./internal/contracttest/... -run TestDecimalFieldsAreJSONNumber -v
//
// ⛔ DSN CHỈ được trỏ vào DB **dùng-một-lần** (`*_test` / `*_tmp`): `testhelpers.New` gọi
// `dropAllTables`. TUYỆT ĐỐI không trỏ vào `DB_NAME` trong `backend/.env` (sự cố 2026-07-18 đã
// mất sạch data 1 DB vì đúng lỗi này). `testhelpers.New` có guard so DSN với `DB_NAME`, nhưng
// guard là lưới cuối chứ không thay kỷ luật.
//
// Không cần server chạy: mọi case gọi thẳng handler qua `gin.CreateTestContext` +
// `httptest.NewRecorder`, không bind port nào.
package contracttest

import (
	"log"
	"os"
	"testing"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	appdb "erpCluvix/db"
	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/helpers"
	"erpCluvix/pkg/logger"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
	// Bắt buộc: production bật cờ này ở main.go:20. Test process KHÔNG chạy main() nên phải
	// bật tay — thiếu dòng này thì decimal.Decimal marshal ra string và lưới sẽ đỏ vì lý do
	// SAI (cấu hình test), che mất lỗi thật cần bắt.
	decimal.MarshalJSONWithoutQuotes = true

	// story-20: `response.ServerError` (nay được `pagination.Paginate` gọi khi query lỗi) ghi
	// `logger.L.Error`. `logger.L` là global nil cho tới khi `logger.Init` chạy — production init
	// ở bootstrap, test process KHÔNG chạy main() nên phải init tay, nếu không case lỗi SQL sẽ
	// nil-panic vì HẠ TẦNG chứ không phải vì hành vi đang kiểm chứng.
	logger.Init(logger.Config{Env: "test", AppCode: "contracttest"})

	s, err := testhelpers.New(testhelpers.MigrationsDir(), testhelpers.SchemaAll)
	if err != nil {
		log.Printf("testhelpers: %v — skipping contract tests", err)
		os.Exit(m.Run())
	}
	defer s.Close()

	testDB = s.DB
	appdb.DB = s.DB

	// Dựng schema map giống bootstrap/app.go — đây chính là cơ chế đang được kiểm chứng.
	helpers.InitJSONColumnMap(s.DB)
	helpers.InitDecimalColumnMap(s.DB)
	if st := helpers.DecimalColumnMapStatusSnapshot(); !st.Ready {
		log.Printf("contracttest: decimalColumnMap NOT ready (%s) — lưới sẽ đỏ vì hạ tầng, không phải vì handler", st.Error)
	}

	os.Exit(m.Run())
}
