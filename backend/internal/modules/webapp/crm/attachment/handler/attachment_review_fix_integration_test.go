//go:build integration

package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"erpCluvix/internal/middleware"
	"erpCluvix/internal/testhelpers"
	"erpCluvix/pkg/i18n"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// fullEngine — như engine() nhưng có middleware giới hạn body thật + route upload theo ref_type.
func (e *attEnv) fullEngine(uid uint, h *Handler) *gin.Engine {
	r := testhelpers.NewGin()
	r.MaxMultipartMemory = middleware.MaxMultipartBodyBytes
	r.Use(middleware.RequestSizeLimit())
	g := r.Group("/api/crm", testhelpers.AuthMiddleware(0, uid))
	g.GET("/customer/:id/attachments", h.ListByCustomer)
	g.POST("/customer/:id/attachments", h.UploadForCustomer)
	g.POST("/attachment", h.Upload)
	g.DELETE("/attachment/:id", h.Delete)
	return r
}

func viMsg(key string, args ...any) string {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	return i18n.TF(c, key, args...)
}

func msgOf(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var b struct {
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b), w.Body.String())
	return b.Message
}

func countRows(t *testing.T, table, where string, args ...any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, testDB.Table(table).Where(where, args...).Count(&n).Error)
	return n
}

// #7 + #9: validate bằng i18n; ref_type ngoài whitelist (kể cả `medical_record` — bảng không tồn
// tại) ⇒ 422, không tạo bản ghi/file nào.
func TestAttachment_UploadValidationI18nAndRefTypeWhitelist(t *testing.T) {
	e := newAttEnv(t)
	owner := testhelpers.CreateUser(t, testDB, "own", nil)
	cid := seedCustomer(t, owner)
	r := e.fullEngine(owner, e.h)

	post := func(fields map[string]string) *httptest.ResponseRecorder {
		body, ct := testhelpers.MultipartBody(t, fields, "file", "a.pdf", []byte(testPDF))
		return do(r, http.MethodPost, "/api/crm/attachment", body, ct)
	}
	cases := []struct {
		name   string
		fields map[string]string
		msg    string
	}{
		{"thiếu ref", map[string]string{"ref_type": "customer"}, viMsg("attachment_ref_required")},
		{"medical_record", map[string]string{"ref_type": "medical_record", "ref_id": fmt.Sprint(cid)}, viMsg("attachment_ref_type_invalid")},
		{"ref_type lạ", map[string]string{"ref_type": "order", "ref_id": fmt.Sprint(cid)}, viMsg("attachment_ref_type_invalid")},
		{"ref_id rác", map[string]string{"ref_type": "customer", "ref_id": fmt.Sprintf("%dabc", cid)}, viMsg("attachment_ref_id_invalid")},
	}
	for _, tc := range cases {
		w := post(tc.fields)
		require.Equal(t, http.StatusUnprocessableEntity, w.Code, tc.name+": "+w.Body.String())
		require.Equal(t, tc.msg, msgOf(t, w), tc.name)
	}
	require.Zero(t, countRows(t, "attachment", "customer_id = ?", cid), "request lỗi không được tạo bản ghi")
	require.Empty(t, testhelpers.ListStoreFiles(t, e.priv.Root()), "request lỗi không được để lại file")

	// Đối chứng: ref_type hợp lệ ⇒ 200.
	w := post(map[string]string{"ref_type": "customer", "ref_id": fmt.Sprint(cid)})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// Thiếu file ⇒ 422 i18n (không còn chuỗi tiếng Việt cứng).
	body, ct := testhelpers.MultipartBody(t, map[string]string{"x": "1"}, "", "", nil)
	w = do(r, http.MethodPost, fmt.Sprintf("/api/crm/customer/%d/attachments", cid), body, ct)
	require.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
	require.Equal(t, viMsg("file_required"), msgOf(t, w))

	// Path id rác ⇒ 404 (helpers.ParseID), không gắn file vào khách `cid`.
	body, ct = testhelpers.MultipartBody(t, nil, "file", "a.pdf", []byte(testPDF))
	require.Equal(t, http.StatusNotFound, do(r, http.MethodPost, fmt.Sprintf("/api/crm/customer/%dabc/attachments", cid), body, ct).Code)
}

// #8: ref_id query phải parse ra số — '5abc' trước đây bị MySQL ép thành 5 và lọc trúng.
func TestAttachment_ListRefIDParsed(t *testing.T) {
	e := newAttEnv(t)
	owner := testhelpers.CreateUser(t, testDB, "own", nil)
	cid := seedCustomer(t, owner)
	r := e.fullEngine(owner, e.h)
	body, ct := testhelpers.MultipartBody(t, nil, "file", "a.pdf", []byte(testPDF))
	require.Equal(t, http.StatusOK, do(r, http.MethodPost, fmt.Sprintf("/api/crm/customer/%d/attachments", cid), body, ct).Code)

	list := func(q string) *httptest.ResponseRecorder {
		return do(r, http.MethodGet, fmt.Sprintf("/api/crm/customer/%d/attachments?ref_type=customer&ref_id=%s", cid, q), nil, "")
	}
	for _, bad := range []string{fmt.Sprintf("%dabc", cid), "abc", "0", "-1"} {
		require.Equal(t, http.StatusNotFound, list(bad).Code, bad)
	}
	w := list(fmt.Sprint(cid))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var res struct {
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	require.Len(t, res.Items, 1)
}

// #10: xoá file đính kèm ghi 1 dòng log_customer (ai xoá, file nào, của khách nào) trong CÙNG
// transaction; xoá file trong kho lỗi ⇒ rollback cả bản ghi lẫn dòng log.
func TestAttachment_DeleteWritesAuditInTx(t *testing.T) {
	e := newAttEnv(t)
	owner := testhelpers.CreateUser(t, testDB, "own", nil)
	cid := seedCustomer(t, owner)
	t.Cleanup(func() { testDB.Exec("DELETE FROM log_customer WHERE customer_id = ?", cid) })
	r := e.fullEngine(owner, e.h)
	upload := func(name string) uint {
		body, ct := testhelpers.MultipartBody(t, nil, "file", name, []byte(testPDF))
		w := do(r, http.MethodPost, fmt.Sprintf("/api/crm/customer/%d/attachments", cid), body, ct)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		return uint(decodeData(t, w)["id"].(float64))
	}

	// Kho lỗi ⇒ 500, bản ghi còn, KHÔNG có dòng log (rollback cùng tx).
	idFail := upload("Ho so loi.pdf")
	broken := New(Deps{DB: testDB, PrivateStore: failingDeleteStore{e.priv}, PublicStore: e.pub})
	require.Equal(t, http.StatusInternalServerError,
		do(e.fullEngine(owner, broken), http.MethodDelete, fmt.Sprintf("/api/crm/attachment/%d", idFail), nil, "").Code)
	require.EqualValues(t, 1, countRows(t, "attachment", "id = ?", idFail))
	require.Zero(t, countRows(t, "log_customer", "customer_id = ?", cid), "rollback ⇒ không còn dòng log")

	// Xoá thành công ⇒ đúng 1 dòng log: người xoá, khách, tên file + id.
	id := upload("Phieu kham.pdf")
	w := do(r, http.MethodDelete, fmt.Sprintf("/api/crm/attachment/%d", id), nil, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var logs []struct {
		ChangedBy     uint   `gorm:"column:changed_by"`
		ActionType    int    `gorm:"column:action_type"`
		ChangedFields string `gorm:"column:changed_fields"`
	}
	require.NoError(t, testDB.Table("log_customer").Select("changed_by, action_type, CAST(changed_fields AS CHAR) AS changed_fields").
		Where("customer_id = ?", cid).Find(&logs).Error)
	require.Len(t, logs, 1)
	require.Equal(t, owner, logs[0].ChangedBy)
	var cf map[string]map[string]any
	require.NoError(t, json.Unmarshal([]byte(logs[0].ChangedFields), &cf), logs[0].ChangedFields)
	require.Equal(t, fmt.Sprintf("Phieu kham.pdf (#%d)", id), cf["attachment"]["from"])
	require.Nil(t, cf["attachment"]["to"])
}

// #4: upload CHUNKED vượt trần body ⇒ 413 i18n, không tạo bản ghi, không để lại file.
func TestAttachment_ChunkedOversizeUpload413(t *testing.T) {
	e := newAttEnv(t)
	owner := testhelpers.CreateUser(t, testDB, "own", nil)
	cid := seedCustomer(t, owner)
	r := e.fullEngine(owner, e.h)

	for _, path := range []string{fmt.Sprintf("/api/crm/customer/%d/attachments", cid), "/api/crm/attachment"} {
		var head strings.Builder
		mw := multipart.NewWriter(&head)
		_ = mw.WriteField("ref_type", "customer")
		_ = mw.WriteField("ref_id", fmt.Sprint(cid))
		_, err := mw.CreateFormFile("file", "big.pdf")
		require.NoError(t, err)
		body := io.MultiReader(strings.NewReader(head.String()+"%PDF-1.4\n"),
			io.LimitReader(repeatReader('a'), middleware.MaxMultipartRequestBytes),
			strings.NewReader("\r\n--"+mw.Boundary()+"--\r\n"))
		req := httptest.NewRequest(http.MethodPost, path, body)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.ContentLength = -1
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusRequestEntityTooLarge, w.Code, path+": "+w.Body.String())
		require.Equal(t, viMsg("payload_too_large"), msgOf(t, w))
	}
	require.Zero(t, countRows(t, "attachment", "customer_id = ?", cid))
	require.Empty(t, testhelpers.ListStoreFiles(t, e.priv.Root()))
}

type repeatReader byte

func (b repeatReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(b)
	}
	return len(p), nil
}
