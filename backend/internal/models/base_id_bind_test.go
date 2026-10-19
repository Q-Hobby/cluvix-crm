// Package models_test — test bind-level cho tag `form:"-"` trên field ID (epic idor-body-id-write,
// story-05).
//
// Lane bị bịt: gin form-binding KHÔNG đọc tag `json`; field thiếu tag `form` thì gin dùng luôn TÊN
// FIELD làm key (`binding/form_mapping.go`: `tryToSetValue` → `field.Name` khi tag rỗng). Field `ID`
// trước đây không có tag `form` ở bất kỳ entity nào → request multipart mang key `ID` ghi đè được
// `item.ID` qua mọi handler có nhánh `c.ShouldBind(&item)`, vô hiệu hoá tenant scope của bước
// `First(&item, routeID)` phía trên.
//
// Bản vá: `form:"-"` trên field `ID` — gin bỏ qua field (`form_mapping.go:85`:
// `if field.Tag.Get(tag) == "-" { return ... }`). Phủ CẢ 2 nhóm entity:
//   - embed base model (`models.BaseModel` / `SimpleModel`...) — vá tại `internal/models/base.go`
//   - tự khai `ID` riêng (203 entity, gồm `config/branch.Branch`) — vá tại từng `dao/entity.go`
//
// Đặt ở external test package (`models_test`) để tránh import cycle: entity dùng làm mẫu đều import
// `erpCluvix/internal/models`. KHÔNG chạm DB (Hard Rule): chỉ dựng request multipart trong bộ nhớ.
package models_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"erpCluvix/internal/models"
	branchdao "erpCluvix/internal/modules/webapp/config/branch/dao"

	"github.com/gin-gonic/gin"
)

// newIDMultipartContext dựng gin.Context mang request multipart/form-data.
// (Tách riêng khỏi helper của base_bind_test.go để 2 file test độc lập nhau.)
func newIDMultipartContext(t *testing.T, fields map[string]string) *gin.Context {
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

// idAttackFields — payload tấn công: thử cả tên field Go (`ID`, lane gin dùng khi thiếu tag `form`)
// lẫn tên json (`id`), kèm 1 field đối chứng để chứng minh request multipart dựng đúng.
func idAttackFields(controlKey, controlVal string) map[string]string {
	return map[string]string{
		"ID":       "99",
		"id":       "99",
		controlKey: controlVal,
	}
}

// simpleModelIDFixture — entity tối thiểu embed models.SimpleModel + field đối chứng có tag form.
// (crm-standalone story-06: thay entity mẫu cũ `zalo/game/dao.ZaloGameGift` đã xoá cùng module zalo;
// thứ được kiểm vẫn là tag `form:"-"` trên ID của base model.)
type simpleModelIDFixture struct {
	models.SimpleModel
	Name string `json:"name" form:"name"`
}

// TestBindIDMultipart_EmbeddedBaseModel — nhóm entity EMBED base model (~125 entity).
// Fixture embed models.SimpleModel → vá tại internal/models/base.go.
func TestBindIDMultipart_EmbeddedBaseModel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c := newIDMultipartContext(t, idAttackFields("name", "Quà test"))

	item := simpleModelIDFixture{}
	item.ID = 5 // mô phỏng First(&item, routeID) đã nạp bản ghi hợp lệ

	if err := c.ShouldBind(&item); err != nil {
		t.Fatalf("ShouldBind trả lỗi: %v", err)
	}

	if item.Name != "Quà test" {
		t.Fatalf("field đối chứng Name không bind được (%q) — request multipart dựng sai, không phải bug ID", item.Name)
	}
	if item.ID != 5 {
		t.Fatalf("ID = %d, muốn 5: client ghi đè được PK qua form key `ID`/`id` (thiếu form:\"-\" trên models base model)", item.ID)
	}
}

// TestBindIDMultipart_OwnIDField — nhóm entity TỰ KHAI ID (203 entity, base model KHÔNG phủ).
// Branch chính là ví dụ trong architecture.md (`PUT /branch/5` + `id=99`).
func TestBindIDMultipart_OwnIDField(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c := newIDMultipartContext(t, idAttackFields("name", "Chi nhánh test"))

	item := branchdao.Branch{ID: 5}

	if err := c.ShouldBind(&item); err != nil {
		t.Fatalf("ShouldBind trả lỗi: %v", err)
	}

	if item.Name != "Chi nhánh test" {
		t.Fatalf("field đối chứng Name không bind được (%q) — request multipart dựng sai, không phải bug ID", item.Name)
	}
	if item.ID != 5 {
		t.Fatalf("ID = %d, muốn 5: client ghi đè được PK qua form key `ID`/`id` (thiếu form:\"-\" trên Branch.ID)", item.ID)
	}
}
