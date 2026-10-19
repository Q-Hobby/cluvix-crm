package handler

import (
	"github.com/gin-gonic/gin"

	"erpCluvix/pkg/query"
)

// Chỉ mục hint cho 2 báo cáo gom `order_service_used` theo `isLock = 1` + dải `created_day`
// (`report-service-used`, `service-type-statistics`). Tất cả có sẵn từ migration TẠO BẢNG
// (20260928093237_create_order_service_used) ⇒ mọi DB CRM đều có, không cần dò information_schema
// trước khi ép/bỏ (chỉ mục vắng mặt = Error 1176 = 500).
const (
	osuDayCoverIndex = "idx_osu_appt_day_cover" // (isLock, created_day, appointment_id, branch_id, totalEnd, payment)

	// osuWideIgnoreIndexes — mọi chỉ mục DẪN ĐẦU `isLock` + FK service_id (xem osuLockedReportTable).
	osuWideIgnoreIndexes = "idx_osu_billing_agg, idx_osu_islock, idx_osu_appt_day_cover, `fk-order_service_used-service_id`"

	// osuHintNarrowRangeSec — dải ngày ≤ 31 ngày ("tháng này"/"tháng trước" của FE) thì ép range trên
	// osuDayCoverIndex; rộng hơn thì quét bảng rẻ hơn tra PK theo chỉ mục (số đo ở dưới).
	osuHintNarrowRangeSec int64 = 31 * 24 * 60 * 60
)

// osuLockedReportTable trả FROM `order_service_used [alias]` kèm index hint theo dải `created_day`.
//
// Nút thắt (bench 982.900 dòng osu / 858.011 dòng isLock=1, `EXPLAIN ANALYZE` 1 lượt, 2026-10-19):
// optimizer chấm `ref` trên chỉ mục dẫn đầu `isLock` (cardinality 2: `idx_osu_billing_agg` /
// `idx_osu_islock` / `idx_osu_appt_day_cover`) RẺ hơn thật nhiều lần ⇒ đọc TOÀN BỘ 858k dòng
// isLock=1 + tra PK từng dòng rồi mới lọc ngày — kể cả khi chỉ lọc 1 tháng:
//   - tháng (78.995 dòng khớp): tự chọn `ref idx_osu_billing_agg` 1.272ms (câu totals) → ép range
//     `(isLock = 1 AND created_day BETWEEN)` trên `idx_osu_appt_day_cover` **459ms**;
//   - 1 năm (454.011 dòng khớp): ép range 1.868ms · bỏ các chỉ mục isLock ⇒ quét bảng **1.234ms**;
//   - không lọc ngày: `ref isLock=1` 2,6s chỉ riêng bước đọc → quét bảng 0,44s.
//
// ⇒ Dải hẹp (cả 2 cận, ≤ 31 ngày): FORCE range. Dải rộng hoặc KHÔNG có bộ lọc ngày: IGNORE các chỉ
// mục dẫn đầu isLock để optimizer về quét bảng — hoặc tự chọn `fk-…-branch_id` / `idx_osu_created_day_branch`
// khi phân bố thật khiến chúng rẻ (vẫn để nó chọn, không ép quét bảng).
// `fk-order_service_used-service_id` cũng bị IGNORE ở nhánh rộng: bỏ join `customer` thì optimizer đổi
// sang `Index scan` trọn chỉ mục service_id để khỏi sort GROUP BY ⇒ 5,6s (đo 1 năm); và nó là đường cho
// plan LẬT "dẫn từ service rồi tra osu theo service_id" (cùng bệnh công nợ dịch vụ). Hai báo cáo này
// không lọc theo osu.service_id nên không mất đường truy cập nào.
//
// Bộ lọc ngày ở dạng khác (chỉ 1 cận, `eq`, `gt/lt`, giá trị không phải số) ⇒ KHÔNG hint, giữ hành vi cũ.
func osuLockedReportTable(c *gin.Context, alias string) string {
	t := "order_service_used"
	if alias != "" {
		t += " " + alias
	}
	if from, to, ok := rangeFilterSec(c, "created_day"); ok {
		if to-from <= osuHintNarrowRangeSec {
			return t + " FORCE INDEX (" + osuDayCoverIndex + ")"
		}
		return t + " IGNORE INDEX (" + osuWideIgnoreIndexes + ")"
	}
	if !hasFilterParam(c, "created_day") {
		return t + " IGNORE INDEX (" + osuWideIgnoreIndexes + ")"
	}
	return t
}

// hasFilterParam — request có gửi BẤT KỲ dạng tham số nào mà query.ApplyFilter đọc cho key này không
// (giá trị đơn/mảng, filter[key], mọi toán tử). Đối chiếu với nhánh đọc của ApplyFilter: thiếu một dạng
// ⇒ coi nhầm "không lọc" ⇒ hint/join sai (kết quả vẫn đúng nếu là hint, SAI nếu là join — xem
// reportServiceUsedNeedsCustomer).
func hasFilterParam(c *gin.Context, key string) bool {
	if len(query.GetFilterValues(c, key)) > 0 || c.Query(key) != "" {
		return true
	}
	for _, op := range []string{"eq", "neq", "like", "is", "nis", "gt", "gte", "lt", "lte"} {
		if query.GetOperatorValue(c, key, op) != "" {
			return true
		}
	}
	return false
}
