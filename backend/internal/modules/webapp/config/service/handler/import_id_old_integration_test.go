//go:build integration

// Integration test — regression cho sự cố thật 2026-09-06 trên DB một khách: import dịch
// vụ chết với `Error 1062 (23000): Duplicate entry ... for key 'service.id_old'` (id_old 365), người
// dùng chỉ thấy nguyên chuỗi SQL: không biết dòng Excel nào sai, cũng không biết dịch vụ nào đang giữ
// giá trị đó.
//
// Test khoá hành vi SAU khi thêm pre-check checkIDOld: 422 kèm số dòng Excel + bản ghi đang giữ,
// và file bị rollback nguyên vẹn.
package handler

import (
	"bytes"
	"encoding/csv"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	authSvc "erpCluvix/internal/modules/webapp/auth/rbac/service"

	"github.com/gin-gonic/gin"
)

// importCtx dựng request multipart đúng như FE gửi (field "file"), nội dung CSV cho dễ đọc —
// readImportRows nhận cả .csv lẫn .xlsx.
func importCtx(t *testing.T, userID uint, csvRows [][]string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	var sheet bytes.Buffer
	sheet.Write([]byte{0xEF, 0xBB, 0xBF}) // BOM giống file export ghi ra
	cw := csv.NewWriter(&sheet)
	for _, r := range csvRows {
		if err := cw.Write(r); err != nil {
			t.Fatal(err)
		}
	}
	cw.Flush()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", "Dich_vu.csv")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(sheet.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/config/service/import-service-from-excel", &body)
	c.Request.Header.Set("Content-Type", mw.FormDataContentType())
	c.Set("userID", userID)
	return c, w
}

func setServiceIDOld(t *testing.T, serviceID uint, idOld int) {
	t.Helper()
	if err := testDB.Exec("UPDATE service SET id_old = ? WHERE id = ?", idOld, serviceID).Error; err != nil {
		t.Fatalf("seed id_old: %v", err)
	}
}

func serviceIDOld(t *testing.T, serviceID uint) *int {
	t.Helper()
	var got *int
	if err := testDB.Raw("SELECT id_old FROM service WHERE id = ?", serviceID).Scan(&got).Error; err != nil {
		t.Fatalf("đọc id_old: %v", err)
	}
	return got
}

func TestImportServiceRejectsDuplicateIDOld(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được coi là pass khi bị bỏ qua")
	}
	userID := seedUser(t)
	grantPerm(t, userID, "manageConfig")
	h := New(Deps{DB: testDB, RbacSvc: &authSvc.RbacService{DB: testDB}})

	owner := seedService(t)  // đang giữ id_old = 365
	target := seedService(t) // dòng import muốn gán 365
	other := seedService(t)
	setServiceIDOld(t, owner, 365)

	id := func(v uint) string { return strconv.FormatUint(uint64(v), 10) }

	t.Run("id cũ đã thuộc dịch vụ khác trong DB → 422 kèm số dòng và bản ghi đang giữ", func(t *testing.T) {
		c, w := importCtx(t, userID, [][]string{
			{"id", "id cũ"},
			{id(target), "365"},
		})
		h.ImportServiceFromExcel(c)

		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("HTTP %d, muốn 422 — %s", w.Code, w.Body.String())
		}
		msg := w.Body.String()
		for _, want := range []string{"dòng 2", "365", id(owner)} {
			if !strings.Contains(msg, want) {
				t.Errorf("thông báo lỗi thiếu %q (người dùng không biết phải sửa gì): %s", want, msg)
			}
		}
		if strings.Contains(msg, "Duplicate entry") {
			t.Errorf("vẫn trả nguyên chuỗi lỗi MySQL cho người dùng: %s", msg)
		}
		if got := serviceIDOld(t, target); got != nil {
			t.Errorf("file phải rollback nguyên vẹn, id_old của target = %d", *got)
		}
	})

	t.Run("hai dòng trong cùng file trùng id cũ → báo đúng cặp dòng", func(t *testing.T) {
		c, w := importCtx(t, userID, [][]string{
			{"id", "id cũ"},
			{id(target), "900"},
			{id(other), "900"},
		})
		h.ImportServiceFromExcel(c)

		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("HTTP %d, muốn 422 — %s", w.Code, w.Body.String())
		}
		msg := w.Body.String()
		// Dòng 3 là dòng sai, dòng 2 là dòng đã dùng trước giá trị đó.
		for _, want := range []string{"dòng 3", "dòng 2", "900"} {
			if !strings.Contains(msg, want) {
				t.Errorf("thông báo lỗi thiếu %q: %s", want, msg)
			}
		}
		if got := serviceIDOld(t, target); got != nil {
			t.Errorf("dòng 2 đã ghi trong transaction phải bị rollback, id_old = %d", *got)
		}
	})

	t.Run("id cũ chưa ai dùng → ghi được, và import lại chính file đó vẫn qua", func(t *testing.T) {
		rows := [][]string{{"id", "id cũ"}, {id(target), "900"}}

		c, w := importCtx(t, userID, rows)
		h.ImportServiceFromExcel(c)
		if w.Code != http.StatusOK {
			t.Fatalf("HTTP %d, muốn 200 — %s", w.Code, w.Body.String())
		}
		got := serviceIDOld(t, target)
		if got == nil || *got != 900 {
			t.Fatalf("id_old sau import = %v, muốn 900", got)
		}

		// Lần 2: bản ghi tự giữ giá trị đó KHÔNG được tính là đụng độ, nếu không thì luồng
		// "export rồi import lại y nguyên" hỏng hoàn toàn.
		c2, w2 := importCtx(t, userID, rows)
		h.ImportServiceFromExcel(c2)
		if w2.Code != http.StatusOK {
			t.Fatalf("import lại cùng file: HTTP %d, muốn 200 — %s", w2.Code, w2.Body.String())
		}
	})
}

// Kịch bản gốc của sự cố thật (2026-09-06): file TỰ NHẤT QUÁN nhưng dòng nhận id_old đứng TRƯỚC dòng trả
// nó ra. Chạy tuần tự thì MySQL nổ UNIQUE ngay tại dòng nhận, dù cuối file không có giá trị nào
// trùng. Pha "nhả id_old" trong ImportServiceFromExcel phải làm file này chạy được, và người dùng
// KHÔNG phải tự sắp lại thứ tự dòng.
func TestImportServiceAllowsIDOldRotationWithinFile(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được coi là pass khi bị bỏ qua")
	}
	userID := seedUser(t)
	grantPerm(t, userID, "manageConfig")
	h := New(Deps{DB: testDB, RbacSvc: &authSvc.RbacService{DB: testDB}})

	id := func(v uint) string { return strconv.FormatUint(uint64(v), 10) }

	t.Run("đổi chỗ id_old giữa hai dòng, dòng nhận đứng trước dòng trả", func(t *testing.T) {
		taker := seedService(t)
		holder := seedService(t)
		setServiceIDOld(t, holder, 365) // holder đang giữ 365 sai; giá trị đúng của nó là 386

		c, w := importCtx(t, userID, [][]string{
			{"id", "id cũ"},
			{id(taker), "365"},  // dòng 2 — nhận
			{id(holder), "386"}, // dòng 3 — mới trả 365 ra
		})
		h.ImportServiceFromExcel(c)

		if w.Code != http.StatusOK {
			t.Fatalf("HTTP %d, muốn 200 — file tự nhất quán không được phụ thuộc thứ tự dòng: %s", w.Code, w.Body.String())
		}
		if got := serviceIDOld(t, taker); got == nil || *got != 365 {
			t.Errorf("taker.id_old = %v, muốn 365", got)
		}
		if got := serviceIDOld(t, holder); got == nil || *got != 386 {
			t.Errorf("holder.id_old = %v, muốn 386", got)
		}
	})

	t.Run("dòng sau XOÁ id_old, dòng trước nhận chính giá trị đó", func(t *testing.T) {
		taker := seedService(t)
		holder := seedService(t)
		setServiceIDOld(t, holder, 273)

		c, w := importCtx(t, userID, [][]string{
			{"id", "id cũ"},
			{id(taker), "273"},
			{id(holder), ""}, // ô trống = xoá về NULL
		})
		h.ImportServiceFromExcel(c)

		if w.Code != http.StatusOK {
			t.Fatalf("HTTP %d, muốn 200 — %s", w.Code, w.Body.String())
		}
		if got := serviceIDOld(t, taker); got == nil || *got != 273 {
			t.Errorf("taker.id_old = %v, muốn 273", got)
		}
		if got := serviceIDOld(t, holder); got != nil {
			t.Errorf("holder.id_old phải về NULL, được %d", *got)
		}
	})

	t.Run("file KHÔNG có cột id cũ → không đụng tới id_old đang lưu", func(t *testing.T) {
		svc := seedService(t)
		setServiceIDOld(t, svc, 777)

		c, w := importCtx(t, userID, [][]string{
			{"id", "Tên dịch vụ"},
			{id(svc), "Tên mới sau import"},
		})
		h.ImportServiceFromExcel(c)

		if w.Code != http.StatusOK {
			t.Fatalf("HTTP %d, muốn 200 — %s", w.Code, w.Body.String())
		}
		if got := serviceIDOld(t, svc); got == nil || *got != 777 {
			t.Fatalf("id_old = %v, muốn giữ nguyên 777 — pha release KHÔNG được chạy khi file thiếu cột 'id cũ'", got)
		}
	})
}
