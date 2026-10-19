//go:build integration

package handler

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"erpCluvix/internal/testhelpers"
)

// callList gọi GET /crm/order-plan qua router thật (middleware tenant giả lập branchActive).
func callList(t *testing.T, branchID uint, q url.Values) (int, map[string]any) {
	t.Helper()
	h := New(Deps{DB: testDB})
	r := testhelpers.NewGin()
	r.Use(testhelpers.AuthMiddleware(branchID, 1))
	r.GET("/order-plan", h.ListOrderPlan)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/order-plan?"+q.Encode(), nil))
	var body map[string]any
	if w.Body.Len() > 0 {
		dec := json.NewDecoder(strings.NewReader(w.Body.String()))
		dec.UseNumber()
		_ = dec.Decode(&body)
	}
	return w.Code, body
}

func listIDs(t *testing.T, body map[string]any) []string {
	t.Helper()
	items, ok := body["items"].([]any)
	require.True(t, ok, "response phải có items, body=%v", body)
	ids := make([]string, 0, len(items))
	for _, it := range items {
		ids = append(ids, fmt.Sprint(it.(map[string]any)["id"]))
	}
	return ids
}

func TestListOrderPlan_LocNgayTuKhoaChiNhanh(t *testing.T) {
	if testDB == nil {
		t.Fatal("TEST_MYSQL_DSN chưa set — integration test này KHÔNG được skip")
	}
	tn := testhelpers.FreshTenant(t, testDB)
	branchB := testhelpers.CreateBranch(t, testDB)

	marker := fmt.Sprintf("KWX%d", time.Now().UnixNano())
	require.NoError(t, testDB.Exec("UPDATE customer SET fullname = ? WHERE id = ?", "Khách "+marker, tn.CustomerID).Error)

	now := time.Now().Unix()
	old := now - 90*24*3600

	o1 := seedOrderPlan(t, tn) // chi nhánh A, hôm nay
	seedOrderService(t, tn, o1, 1_000_000, 0, 0)
	seedOrderService(t, tn, o1, 500_000, 0, 0)

	o2 := seedOrderPlan(t, tn) // chi nhánh A, 90 ngày trước, ghi chú riêng
	require.NoError(t, testDB.Exec("UPDATE order_plan SET created_time = ?, note = ? WHERE id = ?", old, "note-"+marker+"-o2", o2).Error)

	o3 := seedOrderPlan(t, tn) // chi nhánh B — user đang ở A không được thấy
	require.NoError(t, testDB.Exec("UPDATE order_plan SET branch_id = ? WHERE id = ?", branchB, o3).Error)

	id := func(v uint) string { return fmt.Sprint(v) }

	t.Run("ScopeChiNhanh", func(t *testing.T) {
		code, body := callList(t, tn.BranchID, url.Values{"filter[customer_id][in][]": {id(uint(tn.CustomerID))}})
		require.Equal(t, 200, code, "body=%v", body)
		ids := listIDs(t, body)
		require.ElementsMatch(t, []string{id(o1), id(o2)}, ids, "đơn chi nhánh B không được lọt")

		code, body = callList(t, branchB, url.Values{"filter[customer_id][in][]": {id(uint(tn.CustomerID))}})
		require.Equal(t, 200, code)
		require.ElementsMatch(t, []string{id(o3)}, listIDs(t, body))
	})

	t.Run("LocNgay", func(t *testing.T) {
		q := url.Values{
			"filter[customer_id][in][]": {id(uint(tn.CustomerID))},
			"filter[created_time][gte]": {fmt.Sprint(now - 24*3600)},
			"filter[created_time][lte]": {fmt.Sprint(now + 3600)},
		}
		code, body := callList(t, tn.BranchID, q)
		require.Equal(t, 200, code, "body=%v", body)
		require.ElementsMatch(t, []string{id(o1)}, listIDs(t, body))
		require.Equal(t, "1", fmt.Sprint(body["_meta"].(map[string]any)["totalCount"]))
	})

	t.Run("TuKhoaTheoTenKhach", func(t *testing.T) {
		code, body := callList(t, tn.BranchID, url.Values{"_keyword": {marker}})
		require.Equal(t, 200, code, "body=%v", body)
		require.ElementsMatch(t, []string{id(o1), id(o2)}, listIDs(t, body))
	})

	t.Run("TuKhoaTheoGhiChuDon", func(t *testing.T) {
		code, body := callList(t, tn.BranchID, url.Values{"_keyword": {"note-" + marker}})
		require.Equal(t, 200, code, "body=%v", body)
		require.ElementsMatch(t, []string{id(o2)}, listIDs(t, body))
	})

	t.Run("TuKhoaKhongKhop", func(t *testing.T) {
		code, body := callList(t, tn.BranchID, url.Values{"_keyword": {marker + "-khong-co"}})
		require.Equal(t, 200, code, "body=%v", body)
		require.Empty(t, listIDs(t, body))
	})

	t.Run("TongTienVaDichVuCon", func(t *testing.T) {
		code, body := callList(t, tn.BranchID, url.Values{"_keyword": {marker}, "filter[created_time][gte]": {fmt.Sprint(now - 24*3600)}, "filter[created_time][lte]": {fmt.Sprint(now + 3600)}})
		require.Equal(t, 200, code, "body=%v", body)
		items := body["items"].([]any)
		require.Len(t, items, 1)
		row := items[0].(map[string]any)
		require.Equal(t, "1500000", fmt.Sprint(row["total"]))
		require.Len(t, row["orderServiceUseds"].([]any), 2)
		cust := row["customer"].(map[string]any)
		require.Equal(t, id(uint(tn.CustomerID)), fmt.Sprint(cust["id"]))
	})
}
