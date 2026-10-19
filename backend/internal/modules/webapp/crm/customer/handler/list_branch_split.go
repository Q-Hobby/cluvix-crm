package handler

import (
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"erpCluvix/pkg/tenant"
)

// ───────────────────────────────────────────────────────────────────────────────
// Danh sách khách hàng cho nhân viên CHỈ THUỘC 1 CHI NHÁNH (bench C-4, 2026-10-19).
//
// Bệnh: scope `ScopedDBNullBranch` cho 1 chi nhánh sinh `(customer.branch_id = ? OR
// customer.branch_id IS NULL)`. Vế `OR IS NULL` biến điều kiện thành `ref_or_null` trên
// `fk-customer_branch_id` ⇒ hai dải khoá, không còn luồng đã-sắp-theo-`id` nào ⇒ câu trang
// đọc CẢ chi nhánh rồi filesort cho `ORDER BY id DESC LIMIT 15`, và câu COUNT tra dòng theo
// chỉ mục hẹp đó thay vì quét phủ `idx_customer_scope_status (status, branch_id)`.
// Admin 5 chi nhánh (`IN (1..5) OR IS NULL`) KHÔNG dính: optimizer quét ngược PRIMARY rồi dừng sớm.
//
// Chữa (2 phần, CHỈ bật khi phạm vi rút về đúng 1 chi nhánh — `customerListSingleBranch`):
//
//  1. Câu TRANG: tách vế OR thành 2 nhánh `UNION ALL` — `branch_id = B` và `branch_id IS NULL`.
//     Mỗi nhánh là equality ⇒ `(branch_id)` + PK ngầm ở đuôi chỉ mục = `(branch_id, id)` ⇒
//     `Index lookup … (reverse)` dừng sớm ở dòng thứ `offset+perPage`. Lấy id của trang từ hợp
//     2 nhánh rồi mới nạp đủ cột theo PK. KHÔNG dùng index hint nào ⇒ mỗi nhánh optimizer vẫn tự
//     chọn chỉ mục hợp bộ lọc (FULLTEXT cho từ khoá, `idx_customer_scope_status` cho lọc 1
//     trạng thái, `fk-customer_source_id` …), và không có tên chỉ mục nào bị cứng hoá trong code.
//     Có cổng `customerListSplitWorthIt`: trang sâu (offset+perPage > total/2) đi đường cũ.
//  2. Câu COUNT: optimizer hint `NO_INDEX` cho hai chỉ mục dẫn đầu bằng `branch_id`
//     (`fk-customer_branch_id`, `idx_customer_branch_revenue`) ⇒ optimizer rơi về quét PHỦ
//     `idx_customer_scope_status` (28 dải `(status, branch_id)` chính xác — `status IN (14 giá
//     trị)` LUÔN có mặt nên chỉ mục này luôn phủ được vế scope). Có cổng: KHÔNG gắn khi có
//     từ khoá (đường LIKE + hint ⇒ table scan, đo chậm hơn ~10%; đường ngram thì hint vô nghĩa).
//     Hint KHÔNG phải `FORCE/IGNORE INDEX`: chỉ mục bị đổi tên/xoá ⇒ MySQL chỉ cảnh báo 3128
//     rồi bỏ qua hint (đã thử trên 9.7), về đúng plan cũ — không bao giờ lỗi 1176.
//
// ── Bảng plan — EXPLAIN ANALYZE MỘT lượt mỗi ô (không phải chiến dịch đo giờ), trên CHÍNH SQL
//    handler sinh ra (DryRun + nội suy tham số), bench L 500k khách / 5 CN, CN1 = 200.706 dòng
//    (40%), buffer pool 128MB, 2026-10-19. ms = thời gian server; "cũ" = trước bản vá. ─────────
//
//	câu TRANG, scope 1 CN (CN1)   | cũ: plan · ms                                 | mới: plan · ms
//	không lọc (mặc định FE)       | ref_or_null fk-branch + Sort 200k · 392       | UNION: lookup fk-branch(=1) reverse · 0,15
//	lọc trạng thái = 12           | lookup scope_status(12,1|NULL) + Sort · 111   | UNION: lookup scope_status(12,1) reverse · 0,11
//	filter[branch_id] = 1         | lookup fk-branch(=1) reverse · 0,07           | UNION · 0,17
//	filter[branch_id] = 5         | Impossible WHERE                              | Impossible WHERE · 0,006
//	từ khoá ngram, khớp 1 dòng    | Full-text + Sort row IDs · 0,05               | UNION: Full-text / lookup NULL · 0,08
//	từ khoá LIKE "Nguyen"         | ref_or_null + Sort · 413                      | UNION: lookup fk-branch reverse · 0,79
//	từ khoá LIKE không khớp       | ref_or_null (quét CN) · 416                   | UNION (quét CN, không tránh được) · 359
//	created_day 1 tháng           | PRIMARY reverse · 1,3                         | UNION: range fk-branch reverse · 0,80
//	source_id = 3                 | lookup fk-source reverse · 0,15               | UNION · 0,27
//	payment_user_id = 13          | lookup fk-payment_user reverse · 0,11         | UNION · 0,20
//	gender = 1, trang 2           | ref_or_null + Sort · 376                      | UNION · 0,34
//	_ageFrom/_ageTo               | ref_or_null + Sort · 389                      | UNION · 0,16
//	trang 500 (offset 7.485)      | ref_or_null + Sort · 496                      | UNION · 17
//	offset 79.985 (≈ total/2)¹    | ref_or_null + Sort · 391                      | UNION · 338 — điểm hoà ⇒ cổng
//	offset 599.985 (> total)¹     | ref_or_null + Sort · 349                      | UNION 607 ⇒ cổng giữ đường CŨ
//	chi nhánh rỗng (id 999)¹      | ref_or_null · 0,03                            | UNION · 0,09
//	¹ đo trên câu chỉ lấy id (cùng plan), không phải SQL đầy đủ cột.
//
//	câu COUNT, scope 1 CN (CN1)   | cũ                                            | mới
//	không lọc                     | ref_or_null fk-branch, tra dòng · 307         | hint: covering range scope_status · 41
//	filter[branch_id] = 1         | lookup fk-branch, tra dòng · 267              | hint: covering range · 43
//	lọc trạng thái = 12           | covering scope_status · 10,9                  | như cũ · 10,6
//	từ khoá ngram / LIKE          | Full-text 0,04 / ref_or_null 384              | cổng: KHÔNG hint ⇒ như cũ (0,03 / 383)
//	created_day 1 tháng           | MRR created_day_branch · 24,5                 | như cũ · 23,4
//	source_id = 3                 | lookup fk-source · 155                        | như cũ · 154
//	payment_user_id = 13          | lookup fk-payment_user · 35                   | như cũ · 35
//	gender = 1 / _ageFrom-_ageTo  | ref_or_null · 309 / 346                       | table scan · 299 / 304
//
//	Scope admin (5 CN, hoặc không giới hạn CN): cổng `customerListSingleBranch` đóng ⇒ SQL y hệt
//	trước bản vá (đối chiếu 13 tổ hợp trên: count + id + md5 toàn bộ dòng trang = nhau).
//
//	Đối chiếu kết quả trên bench (26 ô = 13 tổ hợp × 2 scope): tổng, số dòng, md5 toàn bộ cột
//	của trang cũ = mới ở MỌI ô. Bench không có khách branch_id NULL ⇒ nhánh IS NULL được phủ bởi
//	list_branch_split_integration_test.go (khách NULL xen kẽ id với khách CN, mọi offset).
//
// ── Biến thể ĐÃ ĐO và LOẠI ─────────────────────────────────────────────────────────────────
//
//	`… AND customer.branch_id IS NOT NULL` gắn thêm vào câu 1 khối   | optimizer KHÔNG rút gọn: vẫn
//	                                                                 | ref_or_null + Sort 245k
//	`NO_INDEX(fk-branch, branch_revenue)` cho CẢ câu trang          | CN1 0,11ms nhưng (a) lọc 1 trạng
//	                                                                 | thái vẫn Sort (15ms); (b) admin lọc
//	                                                                 | `filter[branch_id]` đổi từ lookup
//	                                                                 | fk-branch reverse sang PRIMARY reverse
//	                                                                 | — chi nhánh cũ toàn id thấp sẽ quét
//	                                                                 | gần cả bảng; (c) phụ thuộc cost model
//	                                                                 | đoán phân bố id đều.
//	`FORCE INDEX (PRIMARY)` vô điều kiện (bench C-4: 0,2ms)         | từ khoá/bộ lọc chọn lọc ⇒ quét ngược
//	                                                                 | 500k dòng; chi nhánh nhỏ/cũ cũng vậy.
//	`FORCE INDEX (idx_customer_scope_status)` cho COUNT             | lỗi 1176 khi chỉ mục đổi tên/bị xoá
//	                                                                 | và tắt luôn FULLTEXT ⇒ dùng NO_INDEX.
//	UNION cho COUNT (đếm 2 nhánh rồi cộng)                          | nhánh `branch_id = 1` optimizer VẪN
//	                                                                 | chọn lookup fk-branch tra dòng — vô ích.
// ───────────────────────────────────────────────────────────────────────────────

// ── §search-global (`GET /crm/customer/search-global`, vòng 2 — 2026-10-19) ───────────────────
// Cùng bệnh, cùng thuốc: SearchGlobal tự dựng vế `(branch_id = ? OR branch_id IS NULL)` (không qua
// ScopedDBNullBranch) + 6 cột LIKE `%kw%` + `ORDER BY id DESC LIMIT 50`. Phạm vi đúng 1 chi nhánh
// (và cross-branch TẮT) ⇒ customerSplitByBranch(offset 0, limit 50); còn lại SQL y hệt bản cũ.
// Không có COUNT, không có cổng trang sâu (luôn offset 0). FE topbar chỉ gửi `keyword` (trim,
// debounce 500ms, mọi độ dài ≥1; BE < 2 ký tự ⇒ []), không per-page.
//
//	keyword (CN1, EXPLAIN ANALYZE 1 lượt) | cũ: ref_or_null fk-branch + Sort · ms | mới: UNION · ms | admin 5 CN (không đổi)
//	"Nguyễn" (khớp nhiều)                 | 381                                   | 2,1             | 0,87
//	"Kim Khánh" (khớp vừa)                | 372                                   | 38              | 24
//	"Dương Kim Quân" (hiếm, id rải)       | 373                                   | 251             | 158
//	"0913" (số ngắn)                      | 378                                   | 31              | 17
//	"09" (2 ký tự)                        | 389                                   | 0,49            | 0,21
//	"zzqq" / "Nguyeenx" (không khớp)      | 364 / 381                             | 347 / 346       | 666 / 499
//	"0959500000" (SĐT 10 số, khách CN khác)| 365                                  | 342             | 492
//	"KH0001000" (mã, 1 khách id thấp)     | 367                                   | 384             | 490
//	"KH05" (không khớp trong CN1)         | 385                                   | 354             | 500
//	Đối chiếu 20 ô (10 từ khoá × 2 scope): toàn bộ dòng trả về (md5) cũ = mới.
//
// ⇒ Phần còn lại là quét LIKE `%kw%` KHÔNG TRÁNH ĐƯỢC khi khớp < 50 dòng trong phạm vi (~345ms
// CN1, ~500ms admin cả bảng). Endpoint này CHƯA có đường ngram như danh sách: đo 1 lượt
// `MATCH(search_ngram) AGAINST('"kw"') AND (<6 LIKE>)` ở CN1: "959500000" 0,04ms · "KH05" 0,03ms ·
// "KH0001000" 0,05ms (so với 342–384ms). Chưa làm ở vòng này — cần cổng ngram (`customerKeywordUseNgram`)
// + đường lui lỗi 1191/188 như customerListTotal; từ khoá toàn chữ vẫn phải đi LIKE (bảng list_search.go).

// customerCountNoBranchIndexHint — optimizer hint cho câu COUNT ở phạm vi 1 chi nhánh.
//
// ⚠ PHẢI là MỘT khối NO_INDEX liệt kê cả hai chỉ mục. Viết hai khối
// `NO_INDEX(customer a) NO_INDEX(customer b)` thì MySQL bỏ khối thứ hai (cảnh báo 3126
// "conflicting/duplicated") ⇒ optimizer chuyển sang `idx_customer_branch_revenue` và COUNT CHẬM
// HƠN cả trước bản vá (đo: 317 → 626ms) — đã dính trong lúc làm bản vá này, test DryRun neo lại.
// Tên chỉ mục khớp migration hiện tại; đổi tên/xoá chỉ mục ⇒ hint thành cảnh báo 3128, plan về
// như trước bản vá (chậm lại, KHÔNG sai kết quả, KHÔNG lỗi) — production im lặng, nên
// `TestCustomerCountQuery_HintMatchesPlainCount` (integration, SHOW WARNINGS) là chỗ báo đỏ:
// migration nào đổi/xoá 2 chỉ mục này thì sửa hằng số này cùng lượt.
// KHÔNG phụ thuộc `index_status_customer (status)` (bench mục A-2 đề xuất bỏ) — chỉ cần
// `idx_customer_scope_status (status, branch_id)` còn sống.
const customerCountNoBranchIndexHint = "/*+ NO_INDEX(customer `fk-customer_branch_id`, idx_customer_branch_revenue) */"

// customerListSingleBranch — phạm vi của request có rút về đúng 1 chi nhánh (+ NULL) không.
//
// ⚠ Phải khớp luật của `tenant.applyNullBranchScope`: BranchActiveID > 0 ⇒ `branch_id = ? OR IS
// NULL`; không thì BranchIDs (1 phần tử ⇒ `IN (x) OR IS NULL`, cùng tập). Hai nhánh UNION chỉ
// THU HẸP base đã scope (base vẫn mang nguyên vế tenant) nên không bao giờ lộ dòng ngoài phạm vi;
// nhưng nếu luật scope đổi mà hàm này không đổi theo thì hợp 2 nhánh có thể THIẾU dòng —
// `TestCustomerSingleBranchMatchesTenantScope` canh đúng chỗ đó.
func customerListSingleBranch(c *gin.Context) (uint, bool) {
	b := tenant.BoundsFromContext(c)
	if b.BranchActiveID > 0 {
		return b.BranchActiveID, true
	}
	if len(b.BranchIDs) == 1 && b.BranchIDs[0] > 0 {
		return b.BranchIDs[0], true
	}
	return 0, false
}

// customerListSplitWorthIt — cổng câu trang: chỉ tách UNION khi cửa sổ cần đọc
// (offset+perPage) ≤ nửa tổng. Trang sâu thì mỗi nhánh phải đọc gần hết chi nhánh rồi còn
// vật chất hoá + sort hợp id ⇒ thua đường cũ (bảng plan: điểm hoà ở ~total/2, offset > total
// 607 vs 349ms). Kết quả hai đường như nhau, cổng chỉ chọn đường rẻ hơn.
func customerListSplitWorthIt(total int64, offset, perPage int) bool {
	if offset < 0 || perPage < 1 {
		return false
	}
	window := int64(offset) + int64(perPage)
	return window <= total/2
}

// customerListSplitPage dựng câu trang cho phạm vi 1 chi nhánh:
//
//	SELECT <cột> FROM customer WHERE customer.id IN (
//	  SELECT customer_page.id FROM (
//	    (SELECT customer.id FROM customer WHERE <base> AND customer.branch_id = ?  ORDER BY customer.id DESC LIMIT offset+perPage)
//	    UNION ALL
//	    (SELECT customer.id FROM customer WHERE <base> AND customer.branch_id IS NULL ORDER BY customer.id DESC LIMIT offset+perPage)
//	    ORDER BY id DESC LIMIT perPage OFFSET offset) AS customer_page)
//	ORDER BY customer.id DESC
//
// `base` là CHÍNH bộ lọc câu COUNT đã dùng (scope tenant + telesale + mọi filter + từ khoá) — hai
// nhánh chỉ gắn thêm một vế equality, nên tập dòng = base ∩ (branch_id = B ∪ IS NULL) = base
// (vì base đã chứa `branch_id = B OR IS NULL`). Hai nhánh rời nhau ⇒ UNION ALL không trùng id.
// Câu ngoài chỉ nạp ≤ perPage dòng theo PK từ tập id ĐÃ scope, nên không cần gắn lại vế tenant
// (gắn lại là dụ optimizer quay về ref_or_null — đúng cái bệnh đang chữa).
//
// Tự áp LIMIT/OFFSET ⇒ caller PHẢI dùng `pagination.ResultForPage`, KHÔNG `PaginateWithTotal`
// (áp offset lần nữa ⇒ trang 2 trở đi rỗng).
func customerListSplitPage(db, base *gorm.DB, branchID uint, offset, perPage int) *gorm.DB {
	return customerSplitByBranch(db, base, branchID, offset, perPage, customerListSelect)
}

// customerSplitByBranch — thân dùng chung của câu tách UNION ALL (danh sách + search-global):
// tra id của cửa sổ [offset, offset+limit) theo `id DESC` qua 2 nhánh `branch_id = B` / `IS NULL`
// của CHÍNH `base`, rồi nạp `selectCols` theo PK. Hợp đồng (base phải đã mang vế scope
// `branch_id = B OR IS NULL`, hai nhánh rời nhau, câu ngoài không gắn lại scope): xem doc
// customerListSplitPage. Mọi Select/Order sẵn có trên base bị nhánh ghi đè (chỉ lấy `customer.id`).
func customerSplitByBranch(db, base *gorm.DB, branchID uint, offset, limit int, selectCols string) *gorm.DB {
	window := offset + limit
	part := func(cond string, args ...any) *gorm.DB {
		return base.Session(&gorm.Session{}).Where(cond, args...).
			Select("customer.id").Order("customer.id DESC").Limit(window)
	}
	ids := db.Raw("(?) UNION ALL (?) ORDER BY id DESC LIMIT ? OFFSET ?",
		part("customer.branch_id = ?", branchID), part("customer.branch_id IS NULL"), limit, offset)
	return db.Table("customer").
		Where("customer.id IN (?)", db.Table("(?) AS customer_page", ids).Select("customer_page.id")).
		Select(selectCols).
		Order("customer.id DESC")
}

// customerCountQuery — câu COUNT của danh sách (ghi vào `total`). Gắn hint NO_INDEX khi phạm vi
// là 1 chi nhánh và KHÔNG có từ khoá (cổng + số đo: khối ghi chú đầu file). Mọi đường khác giữ
// nguyên `Count` như trước bản vá. Test DryRun gọi CHÍNH hàm này.
func customerCountQuery(c *gin.Context, base *gorm.DB, total *int64) *gorm.DB {
	// TrimSpace khớp applyCustomerKeyword: từ khoá toàn khoảng trắng = không có từ khoá.
	if _, single := customerListSingleBranch(c); single && strings.TrimSpace(c.Query("_keyword")) == "" {
		return base.Session(&gorm.Session{}).Select(customerCountNoBranchIndexHint + " COUNT(*)").Scan(total)
	}
	return base.Session(&gorm.Session{}).Count(total)
}
