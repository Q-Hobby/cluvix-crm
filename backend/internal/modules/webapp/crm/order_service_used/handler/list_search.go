package handler

import (
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"erpCluvix/pkg/query"
	"erpCluvix/pkg/tenant"
)

// ───────────────────────────────────────────────────────────────────────────────
// Tìm kiếm từ khoá cho màn "Dịch vụ điều trị" (GET /api/crm/order-service-used).
// Số đo trên DB clone schema y hệt, 524.288 dịch vụ điều trị / 200.000 kế hoạch /
// 100.000 khách (2026-09-06); ~40ms trong mỗi số là overhead kết nối mysql client.
//
// ── Đường CŨ tốn gì ────────────────────────────────────────────────────────────
// Có `_keyword` ⇒ gắn 4 LEFT JOIN (customer, service, order_plan, và bảng mã giảm
// giá của đơn — story-10 crm-standalone đã bỏ) rồi `LIKE '%kw%'` trên 8 cột. Dấu `%` đứng đầu ⇒ không
// index nào dùng được ⇒ quét toàn bộ 524k dòng theo `ORDER BY id DESC`, join từng dòng:
//     SELECT 0,09–1,81s · COUNT 1,73–1,86s · SUM 1,74–1,80s  ⇒ ~5,4s SQL cho MỘT lần gõ.
// COUNT và SUM LUÔN rơi vào ca xấu nhất vì không bao giờ thoát sớm được như SELECT.
//
// ── Đường MỚI ─────────────────────────────────────────────────────────────────
// Tra id ra trước bằng vài truy vấn nhỏ trên bảng phụ, rồi lọc bảng chính bằng danh
// sách literal — MySQL dùng index_merge trên các khoá sẵn có:
//     tra id 0,02–0,30s · SELECT+COUNT+SUM 0,001–0,17s mỗi truy vấn  ⇒ tổng ~0,3–0,8s.
// Bỏ luôn 4 join ⇒ kéo theo 2 lỗi nữa được vá, xem phần dưới.
//
// ⚠ ĐÃ THỬ VÀ LOẠI, đừng làm lại:
//   · FULLTEXT ngram trên `order_service_used.code`: 2,4–6,9s — TỆ HƠN cả LIKE. Mã dịch vụ
//     có tiền tố gần như đồng nhất ('SU' + số) nên mọi bigram đều trúng ~toàn bảng, phrase
//     search phải giao rồi verify từng dòng. ngram hợp với tên người, KHÔNG hợp với mã.
//   · `order_service_used.code LIKE '%kw%'` để riêng thành truy vấn tra id: 1,8–3,2s.
//   Cách còn lại — và là cách đang dùng — là quét range tiền tố trên UNIQUE KEY
//   (company_id, code): 8–29ms (Covering index range scan). Xem `codePrefixOnly`.
// ───────────────────────────────────────────────────────────────────────────────

// keywordIDCap — trần số id nạp về cho đường nhanh. Vượt trần ⇒ rơi về đường LIKE cũ
// (chậm nhưng ĐÚNG) thay vì cắt bớt danh sách. Cắt bớt = trả thiếu kết quả im lặng,
// đúng loại bug tệ nhất: không lỗi, không cảnh báo.
//
// Giữ riêng cho package này thay vì dùng chung với `crm/appointment/handler`: hai màn có
// bảng phụ khác nhau nên ngưỡng có thể phải chỉnh lệch nhau.
const keywordIDCap = 10000

// codePrefixOnly — user chốt 2026-09-06: tìm theo `order_service_used.code` khớp từ ĐẦU mã
// thay vì chuỗi con bất kỳ đâu trong mã.
//
// Đổi ngữ nghĩa này là CÓ Ý và người dùng cảm nhận được: gõ "SU00001" hoặc dán trọn mã vẫn
// ra; gõ riêng phần đuôi "12345" thì KHÔNG còn ra. Đánh đổi: 1,8–3,2s → 8–29ms, và không có
// cách thứ ba (xem khối "ĐÃ THỬ VÀ LOẠI" ở trên). 3 cột kia của khách hàng (tên/mã/SĐT) vẫn
// giữ nguyên chuỗi con nhờ chỉ mục ngram.
//
// Đặt false ⇒ quay lại chuỗi con cho cột mã, và trả lại đúng trần ~1,8–3,2s.
const codePrefixOnly = true

// applyTreatmentServiceKeywordFast thử áp bộ lọc từ khoá bằng đường tra-id-trước.
//
// Trả về (query đã gắn điều kiện, true) khi đi được đường nhanh — gọi bên ngoài KHÔNG cần
// gắn join nào. Trả về (query nguyên trạng, false) khi phải rơi về đường LIKE cũ — gọi bên
// ngoài PHẢI tự gắn 4 LEFT JOIN + điều kiện LIKE 8 cột.
func (h *Handler) applyTreatmentServiceKeywordFast(c *gin.Context, q *gorm.DB) (*gorm.DB, bool) {
	kw := strings.TrimSpace(c.Query("_keyword"))
	if kw == "" {
		return q, true // không có từ khoá ⇒ không cần join, coi như xong
	}
	if utf8.RuneCountInString(kw) < query.NgramMinLen {
		return q, false
	}
	// Cụm mà ngram không băm được thành token đáng tin ⇒ CẢ đường nhanh không dùng được
	// (cả customer lẫn note đều tra bằng MATCH). Xem query.NgramPhraseSafe.
	if !query.NgramPhraseSafe(kw) {
		return q, false
	}

	like := "%" + kw + "%"

	// Nháy kép BẮT BUỘC: AGAINST('"0912"' IN BOOLEAN MODE) = tìm đúng chuỗi con đó. Thiếu
	// nháy kép ⇒ ngram tách thành các bigram rời rồi OR ⇒ THỪA kết quả. Nháy kép có sẵn
	// trong từ khoá phải bỏ, không thì cú pháp BOOLEAN MODE vỡ.
	phrase := query.NgramPhrase(kw)

	// ── customer ────────────────────────────────────────────────────────────────
	// KHÔNG scope tenant ở đây — CỐ Ý, giống `crm/appointment/handler/list_search.go`:
	// bản cũ join `customer` trần, mọi lọc tenant nằm ở phía order_service_used.
	//
	// ⚠ MATCH phủ 6 cột của chỉ mục `ft_customer_ngram` (fullname, code, nickname, phone,
	// other_phone, old_code) nhưng bản cũ CHỈ tìm fullname/code/phone. Nếu để nguyên MATCH
	// thì đường nhanh trả THỪA (khớp cả nickname/other_phone/old_code) — kết quả khác đường
	// lui, tức là bug "nhanh mà sai". Vì vậy lọc lại đúng 3 cột bằng LIKE ngay trong truy
	// vấn tra id: MATCH đã thu tập rất nhỏ nên LIKE ở đây gần như miễn phí.
	// ⚠ MATCH đi trên CỘT SINH `customer.search_ngram` (gộp 6 cột, migration 20260906173738),
	// KHÔNG phải chỉ mục 6 cột như bản đầu. Lý do đo được trên 76.999 khách thật: FULLTEXT
	// NHIỀU CỘT bỏ SÓT dòng khi một cột đứng trước là chuỗi RỖNG — "Nguyễn" LIKE 7.419 nhưng
	// MATCH chỉ 6.509; riêng nhóm `fullname = ''` mà tên nằm ở `nickname` chỉ ra 68/977.
	// 6% khách của DB đó có `fullname = ''` ⇒ vô hình với tìm kiếm. `NULL` thì không sao, chỉ
	// chuỗi rỗng mới hỏng. Gộp về một cột (CONCAT_WS + NULLIF) đưa về 13/13 khớp LIKE.
	var custIDs []int64
	if err := h.DB.Table("customer").
		Where("MATCH(search_ngram) AGAINST (? IN BOOLEAN MODE)", phrase).
		Where("fullname LIKE ? OR code LIKE ? OR phone LIKE ?", like, like, like).
		Limit(keywordIDCap+1).
		Pluck("id", &custIDs).Error; err != nil {
		return q, false // lỗi 1191 (thiếu chỉ mục FULLTEXT) ⇒ lùi an toàn về LIKE
	}
	if len(custIDs) > keywordIDCap {
		return q, false
	}

	// ── service ─────────────────────────────────────────────────────────────────
	// Danh mục dịch vụ nhỏ (hàng trăm–hàng nghìn dòng) nên LIKE chuỗi con vẫn rẻ: 1–3ms.
	var svcIDs []int64
	if err := h.DB.Table("service").
		Where("name LIKE ? OR code LIKE ?", like, like).
		Limit(keywordIDCap+1).
		Pluck("id", &svcIDs).Error; err != nil {
		return q, false
	}
	if len(svcIDs) > keywordIDCap {
		return q, false
	}

	// ── order_plan.note ───────────────────────────────────────────────
	// (story-10 crm-standalone: bỏ nguồn thứ hai — mã giảm giá của đơn.)
	// note là text tự do ⇒ đi chỉ mục ngram `ft_otp_note_ngram`
	// (migration 20260906171513). Đo ở 524.288 kế hoạch:
	//     không khớp  0,26–0,32s → 0,001s   ·   khớp thưa (16 dòng)  0,32–0,37s → 0,023–0,030s
	//
	// ⚠ AND `note LIKE` là BẮT BUỘC, không phải cho chắc: ngram bỏ ký tự phân cách nên
	// MATCH('"#12345"') hoá thành tìm '12345' ⇒ THỪA (đo được LIKE 11 · MATCH 16). MATCH đã
	// thu tập rất nhỏ nên LIKE ở đây gần như miễn phí. Chiều SÓT do query.NgramPhraseSafe chặn.
	var planIDs []int64
	if err := h.DB.Table("order_plan").
		Where("MATCH(note) AGAINST (? IN BOOLEAN MODE)", phrase).
		Where("note LIKE ?", like).
		Limit(keywordIDCap+1).
		Pluck("id", &planIDs).Error; err != nil {
		return q, false // lỗi 1191 (thiếu chỉ mục FULLTEXT) ⇒ lùi an toàn về LIKE
	}
	if len(planIDs) > keywordIDCap {
		return q, false
	}

	// ── order_service_used.code ─────────────────────────────────────────────────
	// Scope tenant ở ĐÂY là bắt buộc về mặt hiệu năng, không phải để lọc: UNIQUE KEY là
	// (company_id, code), thiếu vế company_id thì `code LIKE 'kw%'` không range-scan được.
	// Truy vấn ngoài vẫn scope lại đầy đủ nên kết quả không đổi.
	codePattern := kw + "%"
	if !codePrefixOnly {
		codePattern = like
	}
	var osuIDs []int64
	if err := tenant.ScopedDBNullBranch(c, h.DB, "order_service_used").
		Table("order_service_used").
		Where("order_service_used.code LIKE ?", codePattern).
		Limit(keywordIDCap+1).
		Pluck("order_service_used.id", &osuIDs).Error; err != nil {
		return q, false
	}
	if len(osuIDs) > keywordIDCap {
		return q, false
	}

	// Không nguồn nào khớp ⇒ PHẢI trả 0 dòng. Bỏ qua điều kiện = trả về toàn bộ danh sách
	// như thể không tìm gì.
	conds := make([]string, 0, 4)
	args := make([]interface{}, 0, 4)
	if len(custIDs) > 0 {
		conds = append(conds, "order_service_used.customer_id IN ?")
		args = append(args, custIDs)
	}
	if len(svcIDs) > 0 {
		conds = append(conds, "order_service_used.service_id IN ?")
		args = append(args, svcIDs)
	}
	if len(planIDs) > 0 {
		conds = append(conds, "order_service_used.order_plan_id IN ?")
		args = append(args, planIDs)
	}
	if len(osuIDs) > 0 {
		conds = append(conds, "order_service_used.id IN ?")
		args = append(args, osuIDs)
	}
	if len(conds) == 0 {
		return q.Where("1 = 0"), true
	}
	return q.Where(strings.Join(conds, " OR "), args...), true
}
