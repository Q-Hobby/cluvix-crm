//go:build integration

package export

import (
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Nghiệm thu Ở QUY MÔ (story-05). Chỉ chạy khi có `EXPORT_BENCH_DSN` trỏ tới DB bench đã nạp sẵn
// ~524k dòng `order_service_used` — KHÔNG dùng `testhelpers` vì nó `dropAllTables`, sẽ xoá sạch
// bench. DB bench phải có đuôi `_test`/`_tmp` và TUYỆT ĐỐI không phải DB thật.
//
// Đo bằng **số dòng InnoDB đọc** (`Handler_read_*`) chứ không bằng đồng hồ: đại lượng này bất biến
// với tải máy, mà máy dev đã nhiều lần có phiên khác chạy bench làm ms vô nghĩa.
//
// Pool ghim **1 kết nối** để mọi câu lệnh của một lượt export đi chung MỘT session ⇒ bộ đếm session
// đo đúng phần việc của lượt đó, không lẫn của ai khác.
func benchDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("EXPORT_BENCH_DSN")
	if dsn == "" {
		t.Skip("không có EXPORT_BENCH_DSN — bỏ qua nghiệm thu quy mô")
	}
	if !strings.Contains(dsn, "_test") && !strings.Contains(dsn, "_tmp") {
		t.Fatalf("EXPORT_BENCH_DSN phải trỏ DB dùng-một-lần (đuôi _test/_tmp): %q", dsn)
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("mở DB bench: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("lấy sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func handlerRows(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	// CAST SIGNED: `VARIABLE_VALUE` là VARCHAR nên SUM() trả FLOAT, scan thẳng vào int64 sẽ lỗi
	// "converting driver.Value type float64".
	var n int64
	err := db.Raw(`SELECT CAST(COALESCE(SUM(VARIABLE_VALUE),0) AS SIGNED) FROM performance_schema.session_status
		WHERE VARIABLE_NAME IN ('Handler_read_next','Handler_read_rnd_next','Handler_read_key',
		                        'Handler_read_first','Handler_read_prev')`).Scan(&n).Error
	if err != nil {
		t.Fatalf("đọc bộ đếm: %v", err)
	}
	return n
}

// osuExportQuery — hình dạng query của ExportOrderServiceUsed (LEFT JOIN nhiều bảng). Story-62: cập
// nhật theo schema mới (bỏ bảng/cột thương hiệu, bảng coupon và cột listDoctors/listDirectSales không
// còn) — DB bench (EXPORT_BENCH_DSN) phải dựng lại từ `backend/migrations`.
func osuExportQuery(db *gorm.DB) *gorm.DB {
	return db.Table("order_service_used").
		Select(`order_service_used.id, cust.code AS customer_code, cust.old_code AS customer_old_code,
			cust.fullname AS customer_fullname, cust.phone AS customer_phone, br.name AS branch_name,
			otp.code AS plan_code, otp.note AS plan_note,
			order_service_used.code, svc.code AS service_code, svc.name AS service_name,
			svt.name AS service_type_name,
			order_service_used.unit_price, order_service_used.quantity, order_service_used.totalEnd,
			order_service_used.payment, order_service_used.completion_percent,
			order_service_used.created_at, order_service_used.updated_at,
			order_service_used.listRevenues, uc.fullname AS user_created_fullname,
			uu.fullname AS user_updated_fullname, csrc.name AS customer_source_name,
			osrc.name AS source_name, cust.address AS customer_address, cust.year_born,
			cntry.CommonName AS country_name, prov.name AS province_name, ward.name AS ward_name`).
		Joins("LEFT JOIN customer cust ON cust.id = order_service_used.customer_id").
		Joins("LEFT JOIN branch br ON br.id = order_service_used.branch_id").
		Joins("LEFT JOIN order_plan otp ON otp.id = order_service_used.order_plan_id").
		Joins("LEFT JOIN service svc ON svc.id = order_service_used.service_id").
		Joins("LEFT JOIN service_type svt ON svt.id = svc.service_type_id").
		Joins("LEFT JOIN source csrc ON csrc.id = cust.source_id").
		Joins("LEFT JOIN source osrc ON osrc.id = order_service_used.source_id").
		Joins("LEFT JOIN location_country cntry ON cntry.id = cust.country_id").
		Joins("LEFT JOIN location_province prov ON prov.id = cust.province_id").
		Joins("LEFT JOIN location_ward ward ON ward.id = cust.ward_id").
		Joins("LEFT JOIN user_profile uc ON uc.user_id = order_service_used.created_by").
		Joins("LEFT JOIN user_profile uu ON uu.user_id = order_service_used.updated_by").
		Where("order_service_used.isLock = 1")
}

// osuGroupQuery — hình dạng THỨ HAI (AC7): có GROUP BY + ORDER BY, giống ExportAppointmentDebts.
func osuGroupQuery(db *gorm.DB) *gorm.DB {
	return db.Table("order_service_used osu").
		Select(`MIN(osu.id) AS id, osu.customer_id, SUM(osu.totalEnd) AS totalEnd,
			SUM(osu.payment) AS payment, customer.fullname AS customer_fullname`).
		Joins("LEFT JOIN customer ON customer.id = osu.customer_id").
		Where("osu.isLock = 1").
		Group("osu.customer_id, customer.fullname").
		Order("MIN(osu.id) DESC")
}

func TestScale_ExportDocSoDongInnoDB(t *testing.T) {
	db := benchDB(t)

	// Hiệu chuẩn: không có 2 mốc này thì mọi con số bên dưới vô nghĩa.
	if err := db.Exec("FLUSH STATUS").Error; err != nil {
		t.Fatalf("flush status: %v", err)
	}
	if n := handlerRows(t, db); n > 1000 {
		t.Fatalf("sàn bộ đếm quá cao (%d) — nghi có việc khác chen vào cùng session", n)
	}
	var osuTotal int64
	db.Raw("SELECT COUNT(*) FROM order_service_used").Scan(&osuTotal)
	if osuTotal < 500000 {
		t.Fatalf("bench chỉ có %d dòng, cần ≥500.000 (Iron Law: không kết luận từ DB nhỏ)", osuTotal)
	}
	t.Logf("HIỆU CHUẨN: bảng order_service_used = %d dòng", osuTotal)

	for _, tc := range []struct {
		name  string
		build func(*gorm.DB) *gorm.DB
		limit int64
	}{
		{"15 LEFT JOIN (ExportOrderServiceUsed)", osuExportQuery, 8_100_000},
		{"GROUP BY + ORDER BY (ExportAppointmentDebts)", osuGroupQuery, 8_100_000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			var written int64
			cfg := baseCfg(dir, "scale")
			cfg.Headers = []string{"ID"}
			cfg.OnSuccess = func(n int64) { written = n }

			if err := db.Exec("FLUSH STATUS").Error; err != nil {
				t.Fatalf("flush status: %v", err)
			}
			_, err := Export(tc.build(db), cfg, func(r map[string]interface{}) []interface{} {
				return []interface{}{r["id"]}
			})
			if err != nil {
				t.Fatalf("Export lỗi: %v", err)
			}
			got := handlerRows(t, db)

			t.Logf("KẾT QUẢ: %s — %d dòng ra file, ĐỌC %d dòng InnoDB (ngưỡng %d)",
				tc.name, written, got, tc.limit)
			if got > tc.limit {
				t.Fatalf("đọc %d dòng InnoDB, vượt ngưỡng %d", got, tc.limit)
			}
			// Thấp bất thường ⇒ nghi query bị cắt bớt, phải truy chứ không mừng.
			if written < 1000 {
				t.Fatalf("chỉ ra %d dòng — nghi query bị cắt, số đo vô nghĩa", written)
			}
		})
	}
}

// AC4 ở quy mô: ghi đồng thời trong lúc export 262k dòng ⇒ KHÔNG dòng hợp lệ nào bị bỏ sót.
func TestScale_GhiDongThoi_KhongBoSotDong(t *testing.T) {
	db := benchDB(t)
	dsn := os.Getenv("EXPORT_BENCH_DSN")
	writer, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("mở pool ghi: %v", err)
	}
	wDB, _ := writer.DB()
	wDB.SetMaxOpenConns(2)
	// Đóng pool bằng t.Cleanup, KHÔNG bằng defer: `defer` chạy khi hàm test return, tức TRƯỚC mọi
	// t.Cleanup ⇒ lệnh khôi phục dữ liệu ở cleanup phía dưới sẽ bắn vào pool ĐÃ ĐÓNG và im lặng
	// thất bại, để lại 20 dòng `isLock=0` làm hỏng test chạy sau. Cleanup chạy LIFO nên đăng ký
	// đóng pool TRƯỚC = nó chạy SAU cùng.
	t.Cleanup(func() { _ = wDB.Close() })

	const flipped = 20
	// Chọn 20 id đang hợp lệ, nằm rải trong tập kết quả.
	var victims []int64
	if err := writer.Raw(`SELECT id FROM order_service_used
		WHERE isLock = 1
		ORDER BY id LIMIT ?`, flipped).Scan(&victims).Error; err != nil {
		t.Fatalf("chọn dòng để gỡ: %v", err)
	}
	if len(victims) != flipped {
		t.Fatalf("chỉ chọn được %d dòng", len(victims))
	}
	t.Cleanup(func() {
		if err := writer.Exec("UPDATE order_service_used SET isLock = 1 WHERE id IN ?", victims).Error; err != nil {
			t.Errorf("KHÔI PHỤC THẤT BẠI — bench còn %d dòng isLock=0, test sau sẽ sai: %v", len(victims), err)
		}
	})

	dir := t.TempDir()
	calls, idx := 0, 0
	url, err := Export(osuExportQuery(db), baseCfg(dir, "scale-concurrent"),
		func(r map[string]interface{}) []interface{} {
			calls++
			// Rải 20 lượt ghi đều trong suốt quá trình export.
			if idx < flipped && calls%5000 == 1 {
				writer.Exec("UPDATE order_service_used SET isLock = 0 WHERE id = ?", victims[idx])
				idx++
			}
			return []interface{}{r["id"], r["customer_code"], r["customer_fullname"],
				r["service_name"], r["totalEnd"], r["payment"]}
		})
	if err != nil {
		t.Fatalf("Export lỗi: %v", err)
	}
	if idx == 0 {
		t.Fatal("không có lượt ghi nào chen vào — test không chứng minh được gì")
	}
	t.Logf("đã gỡ %d dòng khỏi tập kết quả TRONG LÚC export", idx)

	got := idsFromFile(t, dir, url)
	var stillValid []int64
	if err := writer.Raw(`SELECT id FROM order_service_used
		WHERE isLock = 1`).Scan(&stillValid).Error; err != nil {
		t.Fatalf("đọc tập còn hợp lệ: %v", err)
	}
	var missing []int64
	for _, id := range stillValid {
		if !got[id] {
			missing = append(missing, id)
		}
	}
	t.Logf("file có %d dòng · tập còn hợp lệ %d dòng · THIẾU %d", len(got), len(stillValid), len(missing))
	if len(missing) > 0 {
		show := missing
		if len(show) > 20 {
			show = show[:20]
		}
		t.Fatalf("BỎ SÓT %d dòng đang hợp lệ (20 id đầu: %v)", len(missing), show)
	}
	if len(got) != len(stillValid)+idx {
		t.Logf("ghi chú: file chứa %d dòng = %d còn hợp lệ + %d dòng đã bị gỡ sau khi ghi ra (ảnh chụp nhất quán)",
			len(got), len(stillValid), idx)
	}
}

// ---------------------------------------------------------------------------
// Đo QUERY THẬT của export còn sống (bảng >390k dòng ở khách hiện tại).
//
// Khác với 2 ca trên (đo CƠ CHẾ trên hình dạng query đại diện), ca này chép đúng
// Select/Joins/Where/Order mà handler + hàm Export* thật dựng ra — để trả lời câu
// "endpoint này nay có đủ nhanh chưa", chứ không chỉ "cơ chế đã hết bậc hai chưa".
//
// story-50 crm-standalone (D19): bỏ 2 case bench `treatment_course`/`treatment_course_technical`
// — bảng mồ côi từ story-14, không còn Go code nào (ngoài chính test này) đọc/ghi tới nữa.
// ---------------------------------------------------------------------------

// noteExportQuery — crm/note/handler/find.go GetAllNote + export/config.go ExportNote.
func noteExportQuery(db *gorm.DB) *gorm.DB {
	return db.Table("note").
		Select(`note.id, note.note, note.created_at, uc.fullname as created_by_fullname,
			cust.code as customer_code, cust.fullname as customer_fullname,
			cust.phone as customer_phone`).
		Joins("LEFT JOIN user_profile uc ON uc.user_id = note.created_by").
		Joins("LEFT JOIN customer cust ON cust.id = note.customer_id").
		Order("note.id DESC")
}

func TestScale_QueryThat_BaExportNangNhat(t *testing.T) {
	db := benchDB(t)
	for _, tc := range []struct {
		name  string
		table string
		build func(*gorm.DB) *gorm.DB
	}{
		{"ExportNote (3 join, ORDER BY id DESC = khoá chính)", "note", noteExportQuery},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var tableRows int64
			db.Raw("SELECT COUNT(*) FROM " + tc.table).Scan(&tableRows)

			dir := t.TempDir()
			var written int64
			cfg := baseCfg(dir, "real")
			cfg.Headers = []string{"ID"}
			cfg.OnSuccess = func(n int64) { written = n }

			if err := db.Exec("FLUSH STATUS").Error; err != nil {
				t.Fatalf("flush status: %v", err)
			}
			start := time.Now()
			_, err := Export(tc.build(db), cfg, func(r map[string]interface{}) []interface{} {
				return []interface{}{r["id"]}
			})
			elapsed := time.Since(start)
			if err != nil {
				t.Fatalf("Export lỗi: %v", err)
			}
			reads := handlerRows(t, db)
			var tmpTables, tmpDisk int64
			db.Raw(`SELECT CAST(COALESCE(VARIABLE_VALUE,0) AS SIGNED) FROM performance_schema.session_status
				WHERE VARIABLE_NAME='Created_tmp_tables'`).Scan(&tmpTables)
			db.Raw(`SELECT CAST(COALESCE(VARIABLE_VALUE,0) AS SIGNED) FROM performance_schema.session_status
				WHERE VARIABLE_NAME='Created_tmp_disk_tables'`).Scan(&tmpDisk)

			t.Logf("ĐO THẬT | %s | bảng %d dòng | ra file %d dòng | ĐỌC %d dòng InnoDB | bảng tạm %d (đĩa %d) | %s",
				tc.name, tableRows, written, reads, tmpTables, tmpDisk, elapsed.Round(time.Millisecond))
			if written < 1000 {
				t.Fatalf("chỉ ra %d dòng — nghi query sai, số đo vô nghĩa", written)
			}
		})
	}
}
