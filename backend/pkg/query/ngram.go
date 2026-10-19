package query

import (
	"strings"
	"unicode"
)

// Tiện ích dùng chung cho các màn danh sách tìm kiếm bằng FULLTEXT ngram
// (`crm/appointment`, `crm/treatment_service`, `crm/labo_order`).
//
// CHỈ gom những thứ là THUỘC TÍNH CỦA MYSQL SERVER, giống nhau ở mọi màn. Các ngưỡng
// điều chỉnh theo từng màn — cụ thể là `keywordIDCap` — CỐ Ý giữ riêng trong từng
// package: mỗi màn tra id trên bộ bảng phụ khác nhau nên trần có thể phải chỉnh lệch.
//
// ⚠ Toàn bộ đường ngram CHỈ đúng nhờ các chỉ mục ngram dựng với stopword TẮT
// (migration `20260906122048_rebuild_ngram_omi_and_customer.up.sql`). Parser ngram loại
// token CHỨA stopword, mà danh sách mặc định của MySQL có `a` và `i` một ký tự ⇒ chỉ mục
// dựng sai khiến tìm "Văn", "Mai", "Nam" trả RỖNG mà không có lỗi nào. Cấu hình stopword
// gắn theo BẢNG và bị cố định bởi chỉ mục FULLTEXT ĐẦU TIÊN — thêm một FULLTEXT khác lên
// cùng bảng sau này mà không bỏ hết chỉ mục cũ trước là làm hỏng lại toàn bộ tìm kiếm.

// NgramMinLen — `ngram_token_size` mặc định của MySQL là 2; từ khoá ngắn hơn không sinh
// được token nào nên MATCH trả rỗng. Ca này PHẢI đi đường LIKE cũ.
const NgramMinLen = 2

// NgramPhrase dựng cụm cho `AGAINST(... IN BOOLEAN MODE)`.
//
// Nháy kép BẮT BUỘC: AGAINST('"0912"' IN BOOLEAN MODE) = tìm đúng chuỗi con đó, khớp
// bằng LIKE '%0912%'. Thiếu nháy kép ⇒ ngram tách thành các bigram rời rồi OR ⇒ THỪA
// kết quả. Nháy kép có sẵn trong từ khoá phải bỏ, không thì cú pháp BOOLEAN MODE vỡ.
func NgramPhrase(kw string) string {
	return `"` + strings.ReplaceAll(kw, `"`, "") + `"`
}

// NgramPhraseSafe — MATCH(...) AGAINST('"cụm"' IN BOOLEAN MODE) CHỈ tin được khi mọi "từ"
// trong cụm dài ≥ ngram_token_size. Parser ngram cắt cụm ở mọi ký tự không phải chữ/số, rồi
// băm từng từ thành bigram; từ 1 ký tự không sinh được bigram nào ⇒ CẢ CỤM trượt.
//
// Đo trên 524.288 kế hoạch điều trị / 100.000 khách (2026-09-06) — MATCH trả 0 trong khi
// LIKE trả hàng chục nghìn, không lỗi không cảnh báo:
//
//	"chi nhánh 2"  → note LIKE 37.449  ·  MATCH 0
//	"Văn A"        → customer LIKE 635 ·  MATCH 0
//
// AND thêm LIKE KHÔNG cứu được (0 AND gì cũng là 0) — phải chặn TỪ TRƯỚC, rơi về đường LIKE.
//
// ⚠ Cụm không có ký tự chữ/số nào (vd "—", "--") cũng KHÔNG an toàn: ngram không sinh token
// nào nên MATCH luôn rỗng, còn LIKE thì vẫn khớp.
func NgramPhraseSafe(kw string) bool {
	wordLen := 0
	sawWord := false
	for _, r := range kw {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			wordLen++
			continue
		}
		if wordLen > 0 && wordLen < NgramMinLen {
			return false
		}
		if wordLen > 0 {
			sawWord = true
		}
		wordLen = 0
	}
	if wordLen > 0 {
		if wordLen < NgramMinLen {
			return false
		}
		sawWord = true
	}
	return sawWord
}
