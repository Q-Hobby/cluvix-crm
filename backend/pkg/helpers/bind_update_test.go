package helpers

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"erpCluvix/internal/models"

	"github.com/gin-gonic/gin"
)

// --- Entity giả lập đủ các hình dạng thật gặp trong repo ---

// tuOwnID — entity TỰ KHAI ID (204/329 entity của repo, vd config/branch.Branch).
type tuOwnID struct {
	ID   uint   `json:"id" form:"id"`
	Name string `json:"name" form:"name"`
	Code string `json:"code" form:"code"`
}

// tuPromoted — entity dùng base model THẬT (`models.SimpleModel`): ID là field promoted.
type tuPromoted struct {
	models.SimpleModel
	Name string `json:"name" form:"name"`
}

// tuTenant — nhúng `models.BaseModel` (status + audit): test preserve nhiều field promoted.
type tuTenant struct {
	models.BaseModel
	Name string `json:"name" form:"name"`
}

// tuU64 — khoá uint64 (toàn bộ omnichat/*).
type tuU64 struct {
	ID   uint64 `json:"id" form:"ID"`
	Name string `json:"name" form:"name"`
}

// tuIntID — khoá int.
type tuIntID struct {
	ID   int    `json:"id" form:"id"`
	Name string `json:"name" form:"name"`
}

// tuNoID — struct KHÔNG có field ID (ca "field không tồn tại → không panic").
type tuNoID struct {
	Name string `json:"name" form:"name"`
	Note string `json:"note" form:"note"`
}

func jsonCtx(body string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest(http.MethodPut, "/x", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	return c
}

func multipartCtx(t *testing.T, fields map[string]string) *gin.Context {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("write field %s: %v", k, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest(http.MethodPost, "/x", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	c.Request = req
	return c
}

func intPtr(v int) *int    { return &v }
func uintPtr(v uint) *uint { return &v }

func TestBindUpdate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		ctx      func(t *testing.T) *gin.Context
		dst      func() any
		preserve []string
		wantErr  bool
		check    func(t *testing.T, dst any)
	}{
		{
			// (a) JSON gửi "id": 99 → ID giữ nguyên
			name: "a_json_id_overwrite_blocked",
			ctx:  func(t *testing.T) *gin.Context { return jsonCtx(`{"id":99,"name":"hacked"}`) },
			dst:  func() any { return &tuOwnID{ID: 5, Name: "old", Code: "C1"} },
			check: func(t *testing.T, dst any) {
				got := dst.(*tuOwnID)
				if got.ID != 5 {
					t.Fatalf("ID = %d, want 5 (giữ nguyên)", got.ID)
				}
				if got.Name != "hacked" {
					t.Fatalf("Name = %q, want %q (field khác vẫn ghi)", got.Name, "hacked")
				}
			},
		},
		{
			// (b) multipart form field tên `ID` → giữ nguyên (lane gin form-binding dùng TÊN FIELD)
			name: "b_multipart_ID_overwrite_blocked",
			ctx: func(t *testing.T) *gin.Context {
				return multipartCtx(t, map[string]string{"ID": "99", "id": "99", "name": "hacked", "code": "C2"})
			},
			dst: func() any { return &tuOwnID{ID: 5, Name: "old", Code: "C1"} },
			check: func(t *testing.T, dst any) {
				got := dst.(*tuOwnID)
				if got.ID != 5 {
					t.Fatalf("ID = %d, want 5 (giữ nguyên qua lane multipart)", got.ID)
				}
				if got.Name != "hacked" || got.Code != "C2" {
					t.Fatalf("field khác không được bind: %+v", got)
				}
			},
		},
		{
			// (c) field ID PROMOTED từ models.SimpleModel — lane JSON
			name: "c_promoted_id_json",
			ctx:  func(t *testing.T) *gin.Context { return jsonCtx(`{"id":99,"name":"new","status":0}`) },
			dst: func() any {
				it := &tuPromoted{Name: "old"}
				it.ID = 7
				it.Status = intPtr(1)
				return it
			},
			check: func(t *testing.T, dst any) {
				got := dst.(*tuPromoted)
				if got.ID != 7 {
					t.Fatalf("promoted ID = %d, want 7", got.ID)
				}
				if got.Name != "new" {
					t.Fatalf("Name = %q, want new", got.Name)
				}
				if got.Status == nil || *got.Status != 0 {
					t.Fatalf("Status không được bind: %v", got.Status)
				}
			},
		},
		{
			// (c') field ID PROMOTED — lane multipart (ca dễ hỏng nhất)
			name: "c2_promoted_id_multipart",
			ctx: func(t *testing.T) *gin.Context {
				return multipartCtx(t, map[string]string{"ID": "99", "name": "new"})
			},
			dst: func() any {
				it := &tuPromoted{Name: "old"}
				it.ID = 7
				return it
			},
			check: func(t *testing.T, dst any) {
				got := dst.(*tuPromoted)
				if got.ID != 7 {
					t.Fatalf("promoted ID = %d, want 7 (multipart)", got.ID)
				}
				if got.Name != "new" {
					t.Fatalf("Name = %q, want new", got.Name)
				}
			},
		},
		{
			// (d) preserve nhiều field cùng lúc — gồm cả con trỏ (*uint) promoted từ BaseModel
			name:     "d_preserve_multiple",
			ctx:      func(t *testing.T) *gin.Context { return jsonCtx(`{"id":99,"updated_by":42,"created_by":42,"name":"new"}`) },
			preserve: []string{"UpdatedBy", "CreatedBy"},
			dst: func() any {
				it := &tuTenant{Name: "old"}
				it.ID = 3
				it.UpdatedBy = uintPtr(1)
				it.CreatedBy = uintPtr(2)
				return it
			},
			check: func(t *testing.T, dst any) {
				got := dst.(*tuTenant)
				if got.ID != 3 {
					t.Fatalf("ID = %d, want 3", got.ID)
				}
				if got.UpdatedBy == nil || *got.UpdatedBy != 1 {
					t.Fatalf("UpdatedBy = %v, want 1", got.UpdatedBy)
				}
				if got.CreatedBy == nil || *got.CreatedBy != 2 {
					t.Fatalf("CreatedBy = %v, want 2", got.CreatedBy)
				}
				if got.Name != "new" {
					t.Fatalf("Name = %q, want new", got.Name)
				}
			},
		},
		{
			// (e) khoá uint64 (omnichat/*) — JSON
			name: "e_uint64_json",
			ctx:  func(t *testing.T) *gin.Context { return jsonCtx(`{"id":99,"name":"new"}`) },
			dst:  func() any { return &tuU64{ID: 900000000000000001, Name: "old"} },
			check: func(t *testing.T, dst any) {
				got := dst.(*tuU64)
				if got.ID != 900000000000000001 {
					t.Fatalf("ID = %d, want 900000000000000001", got.ID)
				}
				if got.Name != "new" {
					t.Fatalf("Name = %q, want new", got.Name)
				}
			},
		},
		{
			// (e') khoá uint64 — multipart
			name: "e2_uint64_multipart",
			ctx: func(t *testing.T) *gin.Context {
				return multipartCtx(t, map[string]string{"ID": "99", "name": "new"})
			},
			dst: func() any { return &tuU64{ID: 900000000000000001, Name: "old"} },
			check: func(t *testing.T, dst any) {
				got := dst.(*tuU64)
				if got.ID != 900000000000000001 {
					t.Fatalf("ID = %d, want giữ nguyên (multipart uint64)", got.ID)
				}
			},
		},
		{
			// (e'') khoá int
			name: "e3_int_key",
			ctx:  func(t *testing.T) *gin.Context { return jsonCtx(`{"id":99,"name":"new"}`) },
			dst:  func() any { return &tuIntID{ID: 11, Name: "old"} },
			check: func(t *testing.T, dst any) {
				got := dst.(*tuIntID)
				if got.ID != 11 {
					t.Fatalf("ID = %d, want 11", got.ID)
				}
			},
		},
		{
			// (f) field cần giữ KHÔNG tồn tại trên struct → không panic, bind vẫn chạy
			name:     "f_missing_field_no_panic",
			ctx:      func(t *testing.T) *gin.Context { return jsonCtx(`{"name":"new","note":"n"}`) },
			preserve: []string{"CreatedBy", "ApprovedBy", "KhongCoField"},
			dst:      func() any { return &tuNoID{Name: "old", Note: "o"} },
			check: func(t *testing.T, dst any) {
				got := dst.(*tuNoID)
				if got.Name != "new" || got.Note != "n" {
					t.Fatalf("payload không được bind: %+v", got)
				}
			},
		},
		{
			// (g) payload hợp lệ không gửi id → MỌI field khác vẫn ghi bình thường
			name: "g_normal_payload_all_fields_written",
			ctx:  func(t *testing.T) *gin.Context { return jsonCtx(`{"name":"n2","code":"C9"}`) },
			dst:  func() any { return &tuOwnID{ID: 5, Name: "old", Code: "C1"} },
			check: func(t *testing.T, dst any) {
				got := dst.(*tuOwnID)
				if got.ID != 5 {
					t.Fatalf("ID = %d, want 5", got.ID)
				}
				if got.Name != "n2" || got.Code != "C9" {
					t.Fatalf("field khác bị giữ nhầm: %+v", got)
				}
			},
		},
		{
			// (h) JSON hỏng → trả lỗi như c.ShouldBindJSON, ID vẫn nguyên
			name:    "h_invalid_json_returns_error",
			ctx:     func(t *testing.T) *gin.Context { return jsonCtx(`{"id":99,"name":`) },
			dst:     func() any { return &tuOwnID{ID: 5, Name: "old"} },
			wantErr: true,
			check: func(t *testing.T, dst any) {
				if got := dst.(*tuOwnID); got.ID != 5 {
					t.Fatalf("ID = %d, want 5 kể cả khi bind lỗi", got.ID)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dst := tc.dst()
			err := BindUpdate(tc.ctx(t), dst, tc.preserve...)
			if tc.wantErr != (err != nil) {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			tc.check(t, dst)
		})
	}
}

// TestBindUpdateWith — nhánh binder thủ công (config/branch).
func TestBindUpdateWith(t *testing.T) {
	gin.SetMode(gin.TestMode)

	item := &tuOwnID{ID: 5, Name: "old", Code: "C1"}
	// Binder "hư": cố tình ghi đè ID như một binder mở rộng ẩu có thể làm.
	err := BindUpdateWith(item, func() error {
		item.ID = 99
		item.Name = "new"
		return nil
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if item.ID != 5 {
		t.Fatalf("ID = %d, want 5", item.ID)
	}
	if item.Name != "new" {
		t.Fatalf("Name = %q, want new", item.Name)
	}

	// Lỗi từ binder phải được trả nguyên vẹn.
	sentinel := errors.New("boom")
	if got := BindUpdateWith(item, func() error { return sentinel }); !errors.Is(got, sentinel) {
		t.Fatalf("err = %v, want %v", got, sentinel)
	}
}

// TestBindUpdateBadDst — dst sai kiểu là lỗi lập trình, phải báo rõ, không panic.
func TestBindUpdateBadDst(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []any{nil, tuOwnID{}, (*tuOwnID)(nil), new(int)}
	for _, dst := range cases {
		if err := BindUpdate(jsonCtx(`{}`), dst); !errors.Is(err, ErrBindUpdateDst) {
			t.Fatalf("dst %T: err = %v, want ErrBindUpdateDst", dst, err)
		}
	}
}
