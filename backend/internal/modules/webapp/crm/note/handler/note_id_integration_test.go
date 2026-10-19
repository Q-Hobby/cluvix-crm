//go:build integration

package handler

import (
	"encoding/json"
	"log"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strconv"
	"testing"
	"time"

	appdb "erpCluvix/db"
	"erpCluvix/internal/testhelpers"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
	s, err := testhelpers.New(testhelpers.MigrationsDir(), testhelpers.SchemaAll)
	if err != nil {
		log.Printf("testhelpers: %v — skipping integration tests", err)
		os.Exit(m.Run())
	}
	defer s.Close()
	testDB = s.DB
	appdb.DB = s.DB // pagination.Paginate đọc db.DB global
	os.Exit(m.Run())
}

// seedNote chèn 1 ghi chú của customerID thuộc thương hiệu của tenant và trả id.
func seedNote(t *testing.T, tenant *testhelpers.TestTenant, customerID uint, content string) uint {
	t.Helper()
	now := time.Now().Unix()
	row := testhelpers.WithTenantCols(tenant, map[string]any{
		"note":        content,
		"customer_id": customerID,
		"created_at":  now,
	})
	delete(row, "branch_id") // note KHÔNG có cột branch_id
	require.NoError(t, testDB.Table("note").Create(&row).Error)
	var id uint
	// GORM Create từ map KHÔNG backfill id → đọc lại theo nội dung.
	require.NoError(t, testDB.Table("note").Where("note = ?", content).
		Order("id DESC").Limit(1).Pluck("id", &id).Error)
	require.NotZero(t, id)
	return id
}

func newNoteCtx(t *testing.T, method, target string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, target, nil)
	c.Set("userID", uint(1))
	return c, w
}

// callListAdvise gọi ListAdvise với query `id` ở dạng CHUỖI THÔ (test được cả id rác).
func callListAdvise(t *testing.T, rawID string) *httptest.ResponseRecorder {
	t.Helper()
	c, w := newNoteCtx(t, "GET", "/crm/note/list-advise?id="+url.QueryEscape(rawID))
	(&Handler{DB: testDB}).ListAdvise(c)
	return w
}

// callGetNote gọi GetNote với path param ở dạng CHUỖI THÔ.
func callGetNote(t *testing.T, rawID string) *httptest.ResponseRecorder {
	t.Helper()
	c, w := newNoteCtx(t, "GET", "/crm/note/"+url.PathEscape(rawID))
	c.Params = gin.Params{{Key: "id", Value: rawID}}
	(&Handler{DB: testDB}).GetNote(c)
	return w
}

// callPinNote gọi PinNote với path param ở dạng CHUỖI THÔ + body id (đường fallback CŨ).
func callPinNote(t *testing.T, rawID, bodyID string) *httptest.ResponseRecorder {
	t.Helper()
	target := "/crm/note/" + url.PathEscape(rawID) + "/pin-note?pinned=true"
	if bodyID != "" {
		target += "&id=" + url.QueryEscape(bodyID)
	}
	c, w := newNoteCtx(t, "POST", target)
	c.Params = gin.Params{{Key: "id", Value: rawID}}
	(&Handler{DB: testDB}).PinNote(c)
	return w
}

// callDeleteNote gọi DeleteNote với path param ở dạng CHUỖI THÔ.
func callDeleteNote(t *testing.T, rawID string) *httptest.ResponseRecorder {
	t.Helper()
	c, w := newNoteCtx(t, "DELETE", "/crm/note/"+url.PathEscape(rawID))
	c.Params = gin.Params{{Key: "id", Value: rawID}}
	(&Handler{DB: testDB}).DeleteNote(c)
	return w
}

// id không parse được ra số dương PHẢI bị từ chối, KHÔNG được để MySQL tự ép kiểu.
// Trước bản vá: `Where("customer_id = ?", c.Query("id"))` bind chuỗi thô ⇒
// `?id=<id>abc` trả về đúng ghi chú của khách <id> (warning 1292 bị nuốt).
func TestNoteHandlers_NonNumericIDIsRejected(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được skip")
	}
	tenant := testhelpers.FreshTenant(t, testDB)
	noteID := seedNote(t, tenant, tenant.CustomerID, "Ghi chu kiem tra id rac")
	custRaw := strconv.FormatUint(uint64(tenant.CustomerID), 10)
	noteRaw := strconv.FormatUint(uint64(noteID), 10)

	// Đối chứng: id hợp lệ PHẢI đi lọt — thiếu nó thì mọi assert từ chối bên dưới vô nghĩa.
	t.Run("đối chứng id hợp lệ", func(t *testing.T) {
		w := callListAdvise(t, custRaw)
		require.Equal(t, 200, w.Code)
		var body struct {
			Items []map[string]any `json:"items"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		require.Len(t, body.Items, 1, "phải trả đúng ghi chú vừa seed")

		require.Equal(t, 200, callGetNote(t, noteRaw).Code)
	})

	rubbish := []string{custRaw + "abc", "abc", "", "0", "-1", " " + custRaw}
	for _, raw := range rubbish {
		t.Run("list-advise id="+strconv.Quote(raw), func(t *testing.T) {
			require.Equal(t, 422, callListAdvise(t, raw).Code,
				"customer_id rác phải bị từ chối, KHÔNG được trả ghi chú của khách khác")
		})
	}

	for _, raw := range []string{noteRaw + "abc", "abc", "0", "-1", " " + noteRaw} {
		t.Run("get-note id="+strconv.Quote(raw), func(t *testing.T) {
			require.Equal(t, 404, callGetNote(t, raw).Code)
		})
		t.Run("pin-note id="+strconv.Quote(raw), func(t *testing.T) {
			require.Equal(t, 422, callPinNote(t, raw, "").Code)
		})
		t.Run("delete-note id="+strconv.Quote(raw), func(t *testing.T) {
			require.Equal(t, 404, callDeleteNote(t, raw).Code)
		})
	}
}

// PinNote KHÔNG được lấy id từ body/query khi path param không parse được —
// đường fallback cũ cho phép URL rác + `?id=<note khác>` ghim nhầm ghi chú.
func TestPinNote_IgnoresBodyID(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được skip")
	}
	tenant := testhelpers.FreshTenant(t, testDB)
	victim := seedNote(t, tenant, tenant.CustomerID, "Ghi chu nan nhan pin")

	w := callPinNote(t, "abc", strconv.FormatUint(uint64(victim), 10))
	require.Equal(t, 422, w.Code, "path param rác phải bị từ chối, không rơi về id trong body")

	var exists, pinnedCount int64
	require.NoError(t, testDB.Table("note").Where("id = ?", victim).Count(&exists).Error)
	require.EqualValues(t, 1, exists, "đối chứng: ghi chú nạn nhân phải còn trong DB")
	require.NoError(t, testDB.Table("note").Where("id = ? AND pinned = 1", victim).Count(&pinnedCount).Error)
	require.Zero(t, pinnedCount, "ghi chú KHÔNG được ghim qua id trong body")
}

// Neo cho Hard Rule "KHÔNG SELECT *": response chỉ chứa đúng các cột đã khai báo.
func TestListAdvise_ReturnsOnlyDeclaredColumns(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được skip")
	}
	tenant := testhelpers.FreshTenant(t, testDB)
	seedNote(t, tenant, tenant.CustomerID, "Ghi chu kiem tra cot")

	w := callListAdvise(t, strconv.FormatUint(uint64(tenant.CustomerID), 10))
	require.Equal(t, 200, w.Code)
	var body struct {
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Items, 1)

	got := make([]string, 0, len(body.Items[0]))
	for k := range body.Items[0] {
		got = append(got, k)
	}
	sort.Strings(got)
	// Story-36 (D15): bỏ `company_id` khỏi noteColumns — response không còn đọc company.
	require.Equal(t, []string{
		"created_at", "created_by", "customer_id",
		"id", "note", "pinned", "pinned_at", "pinned_by",
	}, got, "danh sách cột trả về phải khớp hằng noteColumns")
}
