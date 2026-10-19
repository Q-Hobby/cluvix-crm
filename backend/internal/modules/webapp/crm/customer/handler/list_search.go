package handler

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"gorm.io/gorm"

	"erpCluvix/pkg/query"
)

// ───────────────────────────────────────────────────────────────────────────────
// Tối ưu tìm kiếm màn danh sách khách hàng (`GET /crm/customer`; màn Ticket `GET /sale/ticket` đã xoá — D14).
//
// Mọi số dưới đây đo trên DB clone schema y hệt, 615.992 khách / 5.000 CTV (2026-09-07),
// MySQL 9.7, min của 3 lần chạy (máy load 10–23 nên min là ước lượng lúc rảnh nhất).
// Mỗi số là TỔNG một lần mở danh sách = SELECT trang 1 + COUNT của phân trang.
//
// `query.ApplyKeywordFilter` sinh `col LIKE '%kw%'` cho 8 cột. Dấu `%` đứng đầu ⇒ không
// index nào dùng được ⇒ MySQL quét toàn bảng. `SELECT` thoát sớm khi đủ 15 dòng nên ca
// "từ khoá phổ biến" nhìn có vẻ nhanh, nhưng `COUNT` của `pagination` KHÔNG BAO GIỜ thoát
// sớm được ⇒ mọi lần gõ tìm kiếm đều trả giá quét toàn bảng.
//
// ══ VÌ SAO CÓ CỔNG `customerKeywordUseNgram` — ĐỪNG NỚI RỘNG KHI CHƯA ĐO LẠI ══════════
// Đường ngram KHÔNG phải lúc nào cũng thắng. Nó thua ở đúng một nhóm: từ khoá TOÀN CHỮ,
// NGẮN, PHỔ BIẾN — vì các âm tiết tiếng Việt dùng chung bigram nên posting list gần bằng
// cả bảng, và InnoDB FULLTEXT **không** thoát sớm theo `LIMIT` (đã đo: probe
// `MATCH … LIMIT 10001` cho "Nguy" vẫn tốn 1,87s, nên KHÔNG có cách rẻ nào để dò trước
// độ chọn lọc rồi mới quyết định).
//
//	từ khoá        ký tự  LIKE (cũ)   ngram      kết luận
//	"Nguy"           4      1,206s    5,353s     ngram thua 4,4×
//	"Nguyễn"         6      1,483s    2,597s     ngram thua 1,8×
//	"Thị"            3      1,391s    1,950s     ngram thua 1,4×
//	"Trần"           4      1,368s    1,737s     ngram thua 1,3×
//	"09"             2      1,018s    2,861s     ngram thua 2,8×
//	"091"            3      1,458s    1,669s     ngram thua 1,1×
//	"KH4"            3      1,254s    1,929s     ngram thua 1,5×
//	───────────────────────── ranh giới ở 4 ký tự CÓ CHỮ SỐ ──────────────────────────
//	"0913"           4      1,441s    0,593s     ngram thắng 2,4×
//	"KH40"           4      1,628s    0,646s     ngram thắng 2,5×
//	"09135"          5      2,421s    0,546s     ngram thắng 4,4×
//	"KH4028"         6      1,675s    0,404s     ngram thắng 4,1×
//	"0913353112"    10      3,567s    0,657s     ngram thắng 5,4×
//	"Nguyeenx" (gõ sai, không khớp)  2,261s  0,237s   ngram thắng 9,5×
//
// Chữ số tạo bigram entropy cao (số điện thoại, mã KH, mã tin nhắn) nên luôn chọn lọc;
// âm tiết tiếng Việt thì không. Vì vậy cổng là **có ít nhất 1 chữ số VÀ ≥4 ký tự** —
// hai điều kiện đều rút ra từ bảng trên, không phải phỏng đoán. Nhóm bị loại giữ NGUYÊN
// đường LIKE cũ ⇒ không ca nào chậm đi so với hiện tại.
//
// ⚠ Muốn mở cổng cho từ khoá toàn chữ (để ăn ca "gõ sai tên" 9,5×) thì PHẢI đo lại bằng
// bench ≥500k dòng trước — bảng trên nói rõ nó kéo theo ca "Nguyễn" chậm 1,8×.
//
// ══ VÌ SAO VẪN GIỮ `AND (… LIKE …)` SAU KHI ĐÃ CÓ MATCH ═══════════════════════════════
// Không phải "cho chắc". Parser ngram BỎ mọi ký tự phân cách nên MATCH khớp theo chuỗi
// TOKEN kề nhau bất kể phân cách gì — gõ "ID Q1 KH" trả về hàng chục nghìn dòng lẽ ra
// phải rỗng (đo ở `crm/appointment`). LIKE là lưới lọc THỪA. Chi phí gần bằng 0 vì MATCH
// đã thu tập nhỏ trước.
//
// ⚠ Chiều ngược lại (SÓT) thì LIKE KHÔNG cứu được — `0 AND gì` vẫn là 0. Nên mọi cột
// trong `customerKeySearch` BẮT BUỘC phải nằm trong cột sinh `customer.search_ngram`.
// Đó là lý do có migration `20260907081100` (thêm `link_message` + `loai_message`): trước
// nó, 15% khách của một DB thật có `loai_message` là chuỗi toàn số dạng
// '120239609500910566' — đúng loại từ khoá mà cổng cho đi đường nhanh — sẽ VÔ HÌNH.
// Đã đối chiếu 16 từ khoá trên bench: số dòng của `MATCH AND LIKE` khớp CHÍNH XÁC `LIKE`.
// **Thêm cột vào `customerKeySearch` mà không thêm vào cột sinh = sót im lặng.**
// ───────────────────────────────────────────────────────────────────────────────

// customerNgramMinRunes — trần dưới của cổng ngram, đo được ở bảng trên (3 ký tự thua,
// 4 ký tự thắng). Luôn ≥ query.NgramMinLen.
const customerNgramMinRunes = 4

// applyCustomerKeyword áp bộ lọc từ khoá cho danh sách khách hàng.
//
// forceLike = true ⇒ luôn đi đường LIKE cũ, KHÔNG đụng MATCH. Dùng cho (a) đường export và
// (b) lần chạy lại sau khi câu đếm báo lỗi thiếu chỉ mục FULLTEXT — xem listCustomerTotal.
func (h *Handler) applyCustomerKeyword(c query.Params, q *gorm.DB, forceLike bool) *gorm.DB {
	kw := strings.TrimSpace(c.Query("_keyword"))
	if kw == "" {
		return q
	}
	if !forceLike && customerKeywordUseNgram(kw) {
		q = q.Where("MATCH(customer.search_ngram) AGAINST (? IN BOOLEAN MODE)", query.NgramPhrase(kw))
	}
	// LIKE LUÔN được gắn — vừa là đường duy nhất khi cổng đóng, vừa là lưới lọc THỪA của
	// MATCH khi cổng mở. Xem khối ghi chú đầu file.
	return query.ApplyKeywordFilter(c, q, customerKeySearch...)
}

// customerKeywordUseNgram — từ khoá này có đáng đi đường ngram không.
func customerKeywordUseNgram(kw string) bool {
	if utf8.RuneCountInString(kw) < customerNgramMinRunes {
		return false
	}
	// Cụm chứa "từ" 1 ký tự ⇒ ngram không sinh được bigram cho từ đó nên CẢ CỤM trượt,
	// trả rỗng mà không có lỗi. AND LIKE không cứu được (0 AND gì cũng là 0).
	if !query.NgramPhraseSafe(kw) {
		return false
	}
	return strings.ContainsFunc(kw, unicode.IsDigit)
}
