package query

import "testing"

// Gộp từ 2 bộ test trùng nhau trước đây ở `crm/appointment/handler` và
// `crm/treatment_service/handler` (2026-09-07) — KHÔNG bỏ ca nào.
//
// Mọi ca dưới đây lấy TỪ SỐ ĐO thật trên bench schema-clone (2026-09-06), không phải suy
// luận. Cột "vì sao" ghi lại chênh lệch LIKE vs MATCH đo được — đó là bằng chứng cho thấy
// vì sao ca `false` PHẢI rơi về đường LIKE: MATCH trả 0 trong khi LIKE trả hàng nghìn dòng,
// không lỗi không cảnh báo.
func TestNgramPhraseSafe(t *testing.T) {
	cases := []struct {
		kw   string
		want bool
		why  string
	}{
		// ── từ bench 524.288 lịch hẹn / 100.000 khách ──────────────────────────
		{"Nguyễn Văn An", true, "MATCH+LIKE 272 = LIKE 272"},
		{"IB.Q1.LH", true, "mã có dấu chấm, mọi token ≥2 — 87.382 = 87.382"},
		{"IB Q1 LH", true, "gõ sai phân cách: MATCH thuần 87.382 nhưng AND LIKE kéo về đúng 0"},
		{"OLD-0000007", true, "6 = 6"},
		{"0000123", true, "toàn số, 1 token ≥2"},
		{"bé Bo", true, "17.474 = 17.474"},
		{"LH.0101.2", false, "token cuối 1 ký tự"},

		// ── từ bench 524.288 kế hoạch điều trị / 100.000 khách ─────────────────
		{"Thị Mai", true, "MATCH 624 = LIKE 624"},
		{"chi nhánh", true, "MATCH 37449 = LIKE 37449"},
		{"hành 12", true, "từ toàn số vẫn ≥2 ký tự — MATCH 37449 = LIKE 37449"},
		{"bảo hành 12 tháng", true, "MATCH 37449 = LIKE 37449"},
		{"sĩ Hải", true, "'sĩ' đúng 2 ký tự — MATCH 37449 = LIKE 37449"},
		{"#12345", true, "dấu # là phân cách, từ '12345' ≥2 — thừa do ngram được AND LIKE lọc lại"},
		{"— ưu", true, "MATCH 91544 nhưng AND LIKE kéo về đúng 58256 = LIKE"},
		{"Zalo — đã", true, "MATCH 8322 = LIKE 8322"},
		{"09123", true, "toàn số, 1 từ ≥2"},
		{"chi nhánh 2", false, "từ '2' 1 ký tự ⇒ MATCH 0 trong khi LIKE 37449 — SÓT SẠCH"},
		{"nhánh 2", false, "cùng cơ chế — MATCH 0 vs LIKE 37449"},
		{"A Bình", false, "từ 1 ký tự đứng ĐẦU cụm cũng hỏng như đứng cuối"},

		// ── chung cả hai ───────────────────────────────────────────────────────
		{"Văn A", false, "từ 'A' 1 ký tự ⇒ MATCH 0 trong khi customer LIKE 635 — SÓT SẠCH"},
		{"2", false, "một ký tự — MATCH 0 vs LIKE 314545"},
		{"a", false, "một ký tự"},
		{"—", false, "không có ký tự chữ/số nào ⇒ ngram không sinh token, MATCH luôn rỗng"},
		{"--", false, "như trên, và đủ dài để lọt qua NgramMinLen"},
		{"", false, "rỗng"},
	}
	for _, tc := range cases {
		if got := NgramPhraseSafe(tc.kw); got != tc.want {
			t.Errorf("NgramPhraseSafe(%q) = %v, muốn %v — %s", tc.kw, got, tc.want, tc.why)
		}
	}
}

func TestNgramPhrase(t *testing.T) {
	cases := []struct {
		kw, want, why string
	}{
		{`0912`, `"0912"`, "nháy kép BẮT BUỘC: thiếu ⇒ ngram tách bigram rời rồi OR ⇒ THỪA kết quả"},
		{`Nguyễn Văn`, `"Nguyễn Văn"`, "cụm nhiều từ vẫn nằm trong MỘT cặp nháy"},
		{`a"b`, `"ab"`, "nháy kép có sẵn phải bị bỏ, không thì cú pháp BOOLEAN MODE vỡ"},
		{`""`, `""`, "toàn nháy kép ⇒ còn lại cụm rỗng, không sinh cú pháp lỗi"},
	}
	for _, tc := range cases {
		if got := NgramPhrase(tc.kw); got != tc.want {
			t.Errorf("NgramPhrase(%q) = %q, muốn %q — %s", tc.kw, got, tc.want, tc.why)
		}
	}
}
