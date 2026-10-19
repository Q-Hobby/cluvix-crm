// Package models_test — test bind-level cho tag `form:"status"`.
//
// Đặt ở external test package (`models_test`) thay vì `models`: các entity dùng để test
// (crm/appointment/dao, crm/service/dao) đều import `erpCluvix/internal/models`, nên nếu test
// nằm trong chính package `models` sẽ gây import cycle. External test package không nằm trong
// vòng import của package under test → import ngược hợp lệ.
//
// crm-standalone story-06: entity mẫu cũ `zalo/game/dao.ZaloGameGift` xoá cùng module zalo ⇒
// thay bằng struct cục bộ `simpleModelStatusFixture` chỉ embed `models.SimpleModel` (chính là thứ
// được vá). Test `TestBindStatusMultipart_OwnStatusField` (entity `zalo/news/dao.NewSlides` tự khai
// Status) xoá theo entity.
//
// KHÔNG chạm DB (Hard Rule): chỉ dựng request multipart trong bộ nhớ và gọi gin binding.
package models_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"erpCluvix/internal/models"
	appointmentdao "erpCluvix/internal/modules/webapp/crm/appointment/dao"

	"github.com/gin-gonic/gin"
)

// newMultipartContext dựng gin.Context mang request multipart/form-data với các field cho sẵn.
// Mô phỏng đúng cách FE gửi khi form có upload ảnh (utils.service.ts toFormData()).
func newMultipartContext(t *testing.T, fields map[string]string) *gin.Context {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for k, v := range fields {
		if err := writer.WriteField(k, v); err != nil {
			t.Fatalf("write field %q: %v", k, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req
	return c
}

// simpleModelStatusFixture — entity tối thiểu embed models.SimpleModel + 1 field đối chứng có
// tag form, đúng hình dạng các entity thật (Coupon, Material...) đi qua `c.ShouldBind`.
type simpleModelStatusFixture struct {
	models.SimpleModel
	Name string `json:"name" form:"name"`
}

// TestBindStatusMultipart_EmbeddedSimpleModel phủ nhóm entity embed models.SimpleModel
// (Coupon, Material...) — vá tại gốc internal/models/base.go.
func TestBindStatusMultipart_EmbeddedSimpleModel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c := newMultipartContext(t, map[string]string{
		"status": "0",
		"name":   "Quà test",
	})

	var item simpleModelStatusFixture
	if err := c.ShouldBind(&item); err != nil {
		t.Fatalf("ShouldBind trả lỗi: %v", err)
	}

	if item.Name != "Quà test" {
		t.Fatalf("field đối chứng Name không bind được (%q) — request multipart dựng sai, không phải bug status", item.Name)
	}
	if item.Status == nil {
		t.Fatal("Status = nil: gin bỏ qua key `status` (thiếu tag form:\"status\" trên models.SimpleModel)")
	}
	if *item.Status != 0 {
		t.Fatalf("Status = %d, muốn 0", *item.Status)
	}
}

// assertFormTagParity: mọi field có `json:"x"` PHẢI có `form:"x"` cùng tên.
// Lệch tên hoặc thiếu tag => field rụng IM LẶNG khi request tới dạng multipart
// (gin fallback tra tên field Go viết HOA, không khớp key FE gửi) — đúng lớp bug của epic
// be-status-form-tag. Bỏ qua kiểu gin form-binding không hỗ trợ (datatypes.JSON,
// decimal.Decimal): các field đó chỉ nhận được qua JSON body, đã ghi rõ ở deferred-work.md.
func assertFormTagParity(t *testing.T, v any) {
	t.Helper()

	rt := reflect.TypeOf(v)
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		jsonName, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if jsonName == "" || jsonName == "-" {
			continue
		}
		switch f.Type.String() {
		case "datatypes.JSON", "*decimal.Decimal", "decimal.Decimal":
			continue
		}
		formName, _, _ := strings.Cut(f.Tag.Get("form"), ",")
		if formName == "" {
			t.Errorf("%s.%s: có json:%q nhưng THIẾU form tag — sẽ rụng im lặng khi bind multipart",
				rt.Name(), f.Name, jsonName)
			continue
		}
		if formName != jsonName {
			t.Errorf("%s.%s: json:%q nhưng form:%q — lệch tên, FE gửi key json sẽ không bind được",
				rt.Name(), f.Name, jsonName, formName)
		}
	}
}

// TestFormTagParity_ShouldBindEntities khoá nợ 4 của epic be-status-form-tag.
//
// Appointment đi qua `c.ShouldBind` (KHÔNG phải ShouldBindJSON) — xem
// crm/appointment/handler/appointment.go (UpdateAppointment / UpdateAppointmentCancel /
// UpdateConfirmAppointment). `ShouldBind` chọn binder theo Content-Type, nên chỉ cần một
// client gửi multipart là mọi field thiếu tag `form` rụng im lặng. Hiện FE gửi JSON thuần
// nên chưa hỏng; test này giữ cho nó không hỏng khi có client gửi multipart.
//
// Story-65 (D26c): entity `Material` (`crm/service/dao`) xoá — bảng `material` không có trong
// schema mới, chỉ còn được dùng ở test này.
func TestFormTagParity_ShouldBindEntities(t *testing.T) {
	assertFormTagParity(t, appointmentdao.Appointment{})
}
