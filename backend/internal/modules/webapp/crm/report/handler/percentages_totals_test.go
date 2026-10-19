package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// Neo M-20: dòng tổng của TelesalesOnline / EffectiveAdvise (paginateAndRespondWithPercentages)
// cộng bằng decimal — bản cũ `int(ToFloat64)` cắt phần lẻ (.50) của cột tiền. Cột đếm và tỉ lệ %
// không đổi số; tbKhach (tiền) = decimal.

func runPercentages(t *testing.T, rawQuery string, rows []map[string]interface{}, fields []string, cb func(gin.H)) map[string]json.RawMessage {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/x?"+rawQuery, nil)
	paginateAndRespondWithPercentages(c, rows, fields, cb)
	if w.Code != http.StatusOK {
		t.Fatalf("phải 200, got %d", w.Code)
	}
	var body struct {
		Items  []map[string]interface{}   `json:"items"`
		Totals map[string]json.RawMessage `json:"totals"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse JSON: %v — %s", err, w.Body.String())
	}
	return body.Totals
}

func eqTotal(t *testing.T, totals map[string]json.RawMessage, key, want string) {
	t.Helper()
	raw, ok := totals[key]
	if !ok {
		t.Fatalf("thiếu totals.%s", key)
	}
	// Test không chạy main.go (decimal.MarshalJSONWithoutQuotes) ⇒ decimal có thể ra chuỗi có nháy.
	got, err := decimal.NewFromString(strings.Trim(string(raw), `"`))
	if err != nil {
		t.Fatalf("totals.%s không phải JSON number: %s", key, raw)
	}
	if !got.Equal(decimal.RequireFromString(want)) {
		t.Errorf("totals.%s = %s, muốn %s", key, got, want)
	}
}

func telesalesRows() []map[string]interface{} {
	row := func(lead, chamLai, appNew, appNewPlus, app, den, dongY int64, orderPlan, thanhToan, depositMoney string) map[string]interface{} {
		return map[string]interface{}{
			"totalLead": lead, "totalChamLai": chamLai,
			"totalAppointmentNew": appNew, "totalAppointmentNewPlus": appNewPlus,
			"totalAppointment": app, "totalAppointmentDen": den, "totalAppointmentDongY": dongY,
			"totalDeposit":      int64(1),
			"totalOrderPlan":    decimal.RequireFromString(orderPlan),
			"totalThanhToan":    decimal.RequireFromString(thanhToan),
			"totalDepositMoney": decimal.RequireFromString(depositMoney),
		}
	}
	return []map[string]interface{}{
		row(10, 4, 5, 2, 6, 4, 2, "1500000.50", "700000.50", "100000.25"),
		row(10, 0, 3, 1, 4, 2, 1, "1500000.50", "300000.50", "100000.25"),
	}
}

func TestPercentagesTotals_TienKhongBiCatPhanLe(t *testing.T) {
	fields := []string{
		"totalLead", "totalChamLai",
		"totalAppointmentNew", "totalAppointmentNewPlus", "totalAppointment",
		"totalAppointmentDen", "totalAppointmentDongY", "totalDeposit",
		"totalOrderPlan", "totalThanhToan", "totalDepositMoney",
	}
	totals := runPercentages(t, "", telesalesRows(), fields, telesalesTotals)

	// Tiền: bản cũ ra 3000000 / 1000000 / 200000 (cắt .50/.25 từng dòng).
	eqTotal(t, totals, "totalOrderPlan", "3000001")
	eqTotal(t, totals, "totalThanhToan", "1000001")
	eqTotal(t, totals, "totalDepositMoney", "200000.5")
	// Đếm: không đổi số.
	eqTotal(t, totals, "totalLead", "20")
	eqTotal(t, totals, "totalAppointmentDongY", "3")
	eqTotal(t, totals, "totalDeposit", "2")
	// Tỉ lệ %: giống bản cũ (float64 trên số đếm).
	eqTotal(t, totals, "lichMoiLead", "40")                   // 8/20
	eqTotal(t, totals, "lichMoiMinusLead", "25")              // (8-3)/20
	eqTotal(t, totals, "lichMoiPlusChamLai", "75")            // 3/4
	eqTotal(t, totals, "denLichHen", "60")                    // 6/10
	eqTotal(t, totals, "dongYDen", "50")                      // 3/6
	eqTotal(t, totals, "tbKhach", "1000000.3333333333333333") // 3000001/3 — decimal, không cắt
}

func TestPercentagesTotals_KhongCoDongYThiTbKhachBang0(t *testing.T) {
	rows := []map[string]interface{}{{
		"totalLead": int64(0), "totalChamLai": int64(0), "totalAppointmentNew": int64(0),
		"totalAppointmentNewPlus": int64(0), "totalAppointment": int64(0), "totalAppointmentDen": int64(0),
		"totalAppointmentDongY": int64(0), "totalOrderPlan": decimal.RequireFromString("5"),
	}}
	fields := []string{"totalLead", "totalChamLai", "totalAppointmentNew", "totalAppointmentNewPlus",
		"totalAppointment", "totalAppointmentDen", "totalAppointmentDongY", "totalOrderPlan"}
	totals := runPercentages(t, "", rows, fields, telesalesTotals)
	eqTotal(t, totals, "tbKhach", "0")
	eqTotal(t, totals, "denLead", "0")
}

// Sort theo cột tiền phải so cả phần lẻ (bản cũ ToInt64 coi 100.75 == 100.25).
func TestPercentagesTotals_SortTheoDecimal(t *testing.T) {
	rows := []map[string]interface{}{
		{"id": int64(1), "totalThanhToan": decimal.RequireFromString("100.25")},
		{"id": int64(2), "totalThanhToan": decimal.RequireFromString("100.75")},
	}
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/x?sort=-totalThanhToan", nil)
	paginateAndRespondWithPercentages(c, rows, []string{"totalThanhToan"}, nil)
	var body struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) != 2 || body.Items[0].ID != 2 {
		t.Fatalf("sort giảm dần theo totalThanhToan phải ra id 2 trước, got %+v", body.Items)
	}
}

// EffectiveAdvise: totalIsAccepted = tổng isAccepted (số đếm).
func TestPercentagesTotals_EffectiveAdviseGiuSoDem(t *testing.T) {
	rows := []map[string]interface{}{{"isAccepted": int64(3)}, {"isAccepted": int64(4)}}
	totals := runPercentages(t, "", rows, []string{"isAccepted"}, func(tt gin.H) {
		tt["totalIsAccepted"] = tt["isAccepted"]
	})
	eqTotal(t, totals, "totalIsAccepted", "7")
}
