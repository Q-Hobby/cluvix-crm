//go:build integration

package handler

import (
	"log"
	"os"
	"testing"

	"gorm.io/gorm"

	"erpCluvix/db"
	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/helpers"
)

// testDB được TestMain set khi có DB integration; nil nếu không có.
// Mọi test phải tự guard: if testDB == nil { t.Skip(...) }.
var testDB *gorm.DB

func TestMain(m *testing.M) {
	// SchemaAll (KHÔNG phải baseline): endpoint chi tiết đơn select `broker_id`, `listRevenues`,
	// `listDoctors` — 3 cột do migration SAU baseline thêm. Chạy mỗi baseline thì query chết vì
	// Unknown column, tức test đỏ vì sai môi trường chứ không phải vì code sai.
	s, err := testhelpers.New(testhelpers.MigrationsDir(), testhelpers.SchemaAll)
	if err != nil {
		log.Printf("testhelpers: %v — skipping integration tests", err)
		os.Exit(m.Run())
	}
	defer s.Close()
	testDB = s.DB
	// `pagination.Paginate` đọc global `internal/db.DB`, KHÔNG dùng connection truyền vào query —
	// handler List trong package này sẽ query nhầm DB nếu thiếu dòng dưới.
	db.DB = testDB
	// Bootstrap production gọi 2 hàm này (internal/bootstrap/app.go:75-76). Thiếu chúng thì
	// `ParseJSONFieldsForTables`/`ParseDecimalFieldsForTables` là NO-OP câm lặng — test sẽ chạy
	// trên hình dạng response KHÁC production (decimal về dưới dạng chuỗi "62.50").
	helpers.InitJSONColumnMap(testDB)
	helpers.InitDecimalColumnMap(testDB)
	os.Exit(m.Run())
}
