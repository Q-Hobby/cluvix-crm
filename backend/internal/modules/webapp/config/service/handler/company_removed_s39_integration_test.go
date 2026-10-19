//go:build integration

// Story-39/62 (E6/D15, D23): config/service không đọc/lọc/nhận thương hiệu (schema không còn cột đó).
//
//   - Danh mục của dịch vụ (nhóm dịch vụ, đơn vị) thấy đủ mọi dòng, mỗi dòng đúng 1 lần.
//   - Tra danh mục theo TÊN khi import là tra toàn DB: 2 mục trùng tên ⇒ báo mơ hồ (không chọn bừa).
//   - `id_old` kiểm trùng toàn DB (UNIQUE(id_old)).
//   - Round-trip export → import với đúng bộ header ExportService (không còn cột Thương hiệu) chạy
//     được; file cũ còn cột "Thương hiệu" thì cột đó bị bỏ qua.
//   - Create/Update: key `company_id` client (FE cũ) gửi bị bỏ qua (handler xoá khỏi map ghi), không 500.
package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	authSvc "erpCluvix/internal/modules/webapp/auth/rbac/service"
	exportHandlers "erpCluvix/internal/modules/webapp/export"
	"erpCluvix/internal/testhelpers"
)

func s39Seq() string { return strconv.FormatInt(time.Now().UnixNano(), 10) }

func seedServiceTypeRow(t *testing.T, name string) uint {
	t.Helper()
	now := time.Now().Unix()
	row := map[string]any{"name": name, "status": 1, "created_at": now, "updated_at": now}
	if err := testDB.Table("service_type").Create(&row).Error; err != nil {
		t.Fatalf("seed service_type: %v", err)
	}
	id := uint(testhelpers.LastInsertID(testDB))
	t.Cleanup(func() { testDB.Exec("DELETE FROM service_type WHERE id = ?", id) })
	return id
}

func seedUnitRow(t *testing.T, name string) uint {
	t.Helper()
	now := time.Now().Unix()
	row := map[string]any{"name": name, "status": 1, "created_at": now, "updated_at": now}
	if err := testDB.Table("unit").Create(&row).Error; err != nil {
		t.Fatalf("seed unit: %v", err)
	}
	id := uint(testhelpers.LastInsertID(testDB))
	t.Cleanup(func() { testDB.Exec("DELETE FROM unit WHERE id = ?", id) })
	return id
}

// Danh mục trong sheet tra cứu (ServiceCatalogColumns → CatalogSource.Apply): mọi dòng đều có, mỗi
// dòng đúng 1 lần.
func TestServiceCatalog_S39_DuDong(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được coi là pass khi bị bỏ qua")
	}
	exportHandlers.SetDB(testDB)
	seq := s39Seq()

	ids := map[string]map[string]uint{
		"Nhóm dịch vụ": {
			"nhóm 1": seedServiceTypeRow(t, "S39 Nhom1 "+seq),
			"nhóm 2": seedServiceTypeRow(t, "S39 Nhom2 "+seq),
		},
		"Đơn vị tính": {
			"đơn vị 1": seedUnitRow(t, "S39 Dv1 "+seq),
			"đơn vị 2": seedUnitRow(t, "S39 Dv2 "+seq),
		},
	}

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	byTitle := map[string][]string{}
	for _, col := range exportHandlers.ServiceCatalogColumns(c) {
		byTitle[col.Title] = col.Values
	}
	for title, rows := range ids {
		for kind, id := range rows {
			prefix := fmt.Sprintf("%d — ", id)
			n := 0
			for _, v := range byTitle[title] {
				if strings.HasPrefix(v, prefix) {
					n++
				}
			}
			if n != 1 {
				t.Errorf("danh mục %q: dòng %s id=%d xuất hiện %d lần, muốn đúng 1", title, kind, id, n)
			}
		}
	}
}

// Tra tên danh mục + kiểm id_old khi import: phạm vi = toàn DB.
func TestImportService_S39_TraCuuVaKiemTrungToanDB(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được coi là pass khi bị bỏ qua")
	}
	userID := seedUser(t)
	grantPerm(t, userID, "manageConfig")
	h := New(Deps{DB: testDB, RbacSvc: &authSvc.RbacService{DB: testDB}})
	seq := s39Seq()
	id := func(v uint) string { return strconv.FormatUint(uint64(v), 10) }

	svc := seedService(t)

	t.Run("tên nhóm dịch vụ ⇒ nhận đúng nhóm", func(t *testing.T) {
		st := seedServiceTypeRow(t, "S39 Chi "+seq)
		c, w := importCtx(t, userID, [][]string{
			{"id", "Nhóm dịch vụ"},
			{id(svc), "S39 Chi " + seq},
		})
		h.ImportServiceFromExcel(c)
		if w.Code != http.StatusOK {
			t.Fatalf("HTTP %d, muốn 200 — %s", w.Code, w.Body.String())
		}
		var got struct {
			ST *uint `gorm:"column:service_type_id"`
		}
		testDB.Raw("SELECT service_type_id FROM service WHERE id = ?", svc).Scan(&got)
		if got.ST == nil || *got.ST != st {
			t.Errorf("service_type_id = %v, muốn %d", got.ST, st)
		}
	})

	t.Run("id nhóm dịch vụ ⇒ nhận", func(t *testing.T) {
		st := seedServiceTypeRow(t, "S39 Id "+seq)
		c, w := importCtx(t, userID, [][]string{
			{"id", "Nhóm dịch vụ"},
			{id(svc), id(st)},
		})
		h.ImportServiceFromExcel(c)
		if w.Code != http.StatusOK {
			t.Fatalf("HTTP %d, muốn 200 — %s", w.Code, w.Body.String())
		}
	})

	t.Run("2 nhóm trùng tên ⇒ báo mơ hồ, không chọn bừa", func(t *testing.T) {
		name := "S39 Trung " + seq
		seedServiceTypeRow(t, name)
		seedServiceTypeRow(t, name)
		c, w := importCtx(t, userID, [][]string{
			{"id", "Nhóm dịch vụ"},
			{id(svc), name},
		})
		h.ImportServiceFromExcel(c)
		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("HTTP %d, muốn 422 (tên trùng toàn DB) — %s", w.Code, w.Body.String())
		}
	})

	t.Run("id cũ đang thuộc dịch vụ khác ⇒ trùng (UNIQUE toàn DB)", func(t *testing.T) {
		holder := seedService(t)
		setServiceIDOld(t, holder, 51939)
		c, w := importCtx(t, userID, [][]string{
			{"id", "id cũ"},
			{id(svc), "51939"},
		})
		h.ImportServiceFromExcel(c)
		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("HTTP %d, muốn 422 — %s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), id(holder)) {
			t.Errorf("thông báo phải chỉ bản ghi đang giữ id cũ (%d): %s", holder, w.Body.String())
		}
		if got := serviceIDOld(t, svc); got != nil {
			t.Errorf("file phải rollback, id_old = %d", *got)
		}
	})
}

// Round-trip: header thật của ExportService (2 dòng: nhãn + field key, KHÔNG còn cột Thương hiệu)
// → import: dòng cập nhật + dòng tạo mới đều qua. File mẫu cũ còn cột "Thương hiệu" ⇒ cột bị bỏ qua
// (D23), không lỗi.
func TestImportService_S39_RoundTripKhongCotThuongHieu(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được coi là pass khi bị bỏ qua")
	}
	userID := seedUser(t)
	grantPerm(t, userID, "manageConfig")
	h := New(Deps{DB: testDB, RbacSvc: &authSvc.RbacService{DB: testDB}})
	seq := s39Seq()
	ensureServiceConfigCode(t)

	headers := exportHandlers.ServiceExportHeaders()
	keys := exportHandlers.ServiceExportFieldKeys()
	for i := range keys {
		if headers[i] == "Thương hiệu" {
			t.Fatalf("file export còn cột Thương hiệu (%q/%q) — D23 đã bỏ", headers[i], keys[i])
		}
	}

	svc := seedService(t)
	st := seedServiceTypeRow(t, "S39 RT "+seq)
	newName := "S39 Mới " + seq

	valueOf := func(key string, create bool) string {
		switch key {
		case "id":
			if create {
				return ""
			}
			return strconv.FormatUint(uint64(svc), 10)
		case "name":
			if create {
				return newName
			}
			return "S39 Cập nhật " + seq
		case "service_type_id":
			return exportHandlers.CatalogValue(st, "S39 RT "+seq)
		}
		return "" // cột khác để trống: keep-on-empty / clear-on-empty như file export thật
	}
	rows := [][]string{headers, keys}
	for _, create := range []bool{false, true} {
		r := make([]string, len(keys))
		for i, k := range keys {
			r[i] = valueOf(k, create)
		}
		rows = append(rows, r)
	}

	c, w := importCtx(t, userID, rows)
	h.ImportServiceFromExcel(c)
	if w.Code != http.StatusOK {
		t.Fatalf("round-trip import: HTTP %d, muốn 200 — %s", w.Code, w.Body.String())
	}
	var res struct {
		Data struct{ Created, Updated int } `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if res.Data.Created != 1 || res.Data.Updated != 1 {
		t.Fatalf("created=%d updated=%d, muốn 1/1 — %s", res.Data.Created, res.Data.Updated, w.Body.String())
	}
	var updName string
	testDB.Raw("SELECT name FROM service WHERE id = ?", svc).Scan(&updName)
	if updName != "S39 Cập nhật "+seq {
		t.Errorf("dòng cập nhật name = %q, muốn %q", updName, "S39 Cập nhật "+seq)
	}
	var created struct {
		ID   uint   `gorm:"column:id"`
		Code string `gorm:"column:code"`
		ST   *uint  `gorm:"column:service_type_id"`
	}
	testDB.Raw("SELECT id, code, service_type_id FROM service WHERE name = ?", newName).Scan(&created)
	if created.ID == 0 {
		t.Fatal("dòng tạo mới không có trong DB")
	}
	t.Cleanup(func() { testDB.Exec("DELETE FROM service WHERE id = ?", created.ID) })
	if created.Code == "" {
		t.Errorf("dịch vụ mới phải được sinh mã (codegen)")
	}
	if created.ST == nil || *created.ST != st {
		t.Errorf("dịch vụ mới service_type_id = %v, muốn %d", created.ST, st)
	}

	t.Run("file cũ còn cột Thương hiệu ⇒ bỏ qua cột", func(t *testing.T) {
		c, w := importCtx(t, userID, [][]string{
			{"id", "Tên dịch vụ", "Thương hiệu"},
			{strconv.FormatUint(uint64(svc), 10), "S39 Có cột TH " + seq, "7"},
		})
		h.ImportServiceFromExcel(c)
		if w.Code != http.StatusOK {
			t.Fatalf("HTTP %d, muốn 200 — %s", w.Code, w.Body.String())
		}
		var name string
		testDB.Raw("SELECT name FROM service WHERE id = ?", svc).Scan(&name)
		if name != "S39 Có cột TH "+seq {
			t.Errorf("name = %q — các cột khác của file cũ phải vẫn được ghi", name)
		}
	})
}

// Create (JSON + multipart) chạy không cần gì ngoài dữ liệu dịch vụ; key `company_id` do FE cũ gửi
// (JSON create / update) bị handler loại khỏi map ghi ⇒ không 500 "Unknown column", field khác vẫn lưu.
func TestServiceWrite_S39_BoQuaKeyFECompat(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được coi là pass khi bị bỏ qua")
	}
	userID := seedUser(t)
	ensureServiceConfigCode(t)
	h := New(Deps{DB: testDB})
	seq := s39Seq()
	gin.SetMode(gin.TestMode)

	createdID := func(name string) uint {
		var id uint
		testDB.Raw("SELECT id FROM service WHERE name = ?", name).Scan(&id)
		if id > 0 {
			t.Cleanup(func() { testDB.Exec("DELETE FROM service WHERE id = ?", id) })
		}
		return id
	}

	t.Run("JSON kèm key FE-compat ⇒ 201, key bị bỏ qua", func(t *testing.T) {
		name := "S39 JSON " + seq
		raw, _ := json.Marshal(map[string]any{"name": name, "company_id": 99})
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/config/service", bytes.NewReader(raw))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("userID", userID)
		h.CreateService(c)
		if w.Code != http.StatusCreated {
			t.Fatalf("HTTP %d, muốn 201 — %s", w.Code, w.Body.String())
		}
		if id := createdID(name); id == 0 {
			t.Fatal("không thấy dịch vụ vừa tạo")
		}
	})

	t.Run("multipart ⇒ 201", func(t *testing.T) {
		name := "S39 MP " + seq
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		_ = mw.WriteField("name", name)
		_ = mw.Close()
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/config/service/create-service", &body)
		c.Request.Header.Set("Content-Type", mw.FormDataContentType())
		c.Set("userID", userID)
		h.CreateService(c)
		if w.Code != http.StatusCreated {
			t.Fatalf("HTTP %d, muốn 201 — %s", w.Code, w.Body.String())
		}
		if id := createdID(name); id == 0 {
			t.Fatal("không thấy dịch vụ vừa tạo")
		}
	})

	t.Run("Update kèm key FE-compat ⇒ bỏ qua, field khác vẫn lưu", func(t *testing.T) {
		svc := seedService(t)
		c, w := updateCtx(t, svc, userID, map[string]any{
			"updated_at": svcUpdatedAt(t, svc), "name": "S39 Đổi tên " + seq, "company_id": 99,
		})
		h.UpdateService(c)
		if w.Code != http.StatusOK {
			t.Fatalf("HTTP %d, muốn 200 — %s", w.Code, w.Body.String())
		}
		var name string
		testDB.Raw("SELECT name FROM service WHERE id = ?", svc).Scan(&name)
		if name != "S39 Đổi tên "+seq {
			t.Errorf("name = %q, muốn lưu được", name)
		}
	})
}
