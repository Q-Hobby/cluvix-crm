package logger

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Dedup alert Telegram — thêm 2026-09-04 sau sự cố spam 600 tin trong 25 phút.
//
// Bối cảnh: 1 cột thiếu trong DB (`is_identity_verified`, migration chưa apply) làm cron
// đồng bộ zalo_personal fail 100 INSERT mỗi 5 phút. Mỗi `L.Error` = 1 tin Telegram riêng
// ⇒ 600 tin, nội dung gần như y hệt nhau.
//
// ⚠ Vì sao vân tay CHỈ lấy dòng đầu: message SQL Error có dạng
//
//	SQL Error: Error 1054 (42S22): Unknown column 'x' in 'field list'
//	Query: INSERT INTO ... VALUES (<giá trị khác nhau mỗi lần>)
//	Rows: 0 | Time: 792µs
//
// Hash CẢ message thì 600 tin ra 600 vân tay khác nhau (giá trị INSERT khác nhau) → dedup
// vô tác dụng đúng lúc cần nhất. Dòng đầu là phần ổn định: cùng loại lỗi ⇒ cùng vân tay.
//
// Nguyên tắc KHÔNG nuốt tin: tin đầu của mỗi vân tay LUÔN gửi ngay. Các tin trùng trong cửa
// sổ chỉ bị hoãn, và số lượng bị nén được báo kèm tin kế tiếp — không có error nào biến mất
// không dấu vết. Đây là điều kiện để không phá vỡ chủ đích cũ ("theo dõi triệt để mọi error").
const (
	// dedupWindow — trong cửa sổ này, mỗi vân tay chỉ gửi 1 tin. Chọn 5 phút để khớp nhịp cron
	// 5 phút của repo: một nhịp lỗi hàng loạt gom đúng vào 1 tin.
	dedupWindow = 5 * time.Minute
	// dedupMaxEntries — trần số vân tay theo dõi đồng thời. Vượt trần thì dọn hết entry đã hết
	// hạn; vẫn vượt thì reset sạch (thà mất trạng thái nén còn hơn map phình vô hạn trong
	// process chạy dài ngày).
	dedupMaxEntries = 512
)

type dedupEntry struct {
	windowStart time.Time
	suppressed  int
}

var (
	dedupMu    sync.Mutex
	dedupState = make(map[string]*dedupEntry, 64)
)

// dedupFingerprint lấy dòng đầu của message làm vân tay (xem lý do ở comment khối trên).
func dedupFingerprint(msg string) string {
	if i := strings.IndexByte(msg, '\n'); i >= 0 {
		return msg[:i]
	}
	return msg
}

// shouldSendAlert quyết định có gửi tin này không.
//
// Trả về (true, "") — gửi bình thường;
// (true, note) — gửi kèm ghi chú số tin đã bị nén ở cửa sổ trước;
// (false, "") — nén lại, chỉ tăng bộ đếm.
//
// now truyền vào (không gọi time.Now bên trong) để test kiểm được ranh giới cửa sổ.
func shouldSendAlert(msg string, now time.Time) (bool, string) {
	fp := dedupFingerprint(msg)

	dedupMu.Lock()
	defer dedupMu.Unlock()

	e, ok := dedupState[fp]
	if !ok {
		if len(dedupState) >= dedupMaxEntries {
			evictExpiredLocked(now)
		}
		dedupState[fp] = &dedupEntry{windowStart: now}
		return true, ""
	}

	if now.Sub(e.windowStart) < dedupWindow {
		e.suppressed++
		return false, ""
	}

	// Cửa sổ đã hết: mở cửa sổ mới, báo kèm số tin bị nén ở cửa sổ vừa rồi.
	n := e.suppressed
	e.windowStart = now
	e.suppressed = 0
	if n == 0 {
		return true, ""
	}
	return true, fmt.Sprintf("\n🔁 (đã nén %d tin trùng trong %s trước đó)", n, dedupWindow)
}

// evictExpiredLocked dọn entry đã quá 2 cửa sổ. Gọi khi đang giữ dedupMu.
func evictExpiredLocked(now time.Time) {
	for k, v := range dedupState {
		if now.Sub(v.windowStart) > 2*dedupWindow {
			delete(dedupState, k)
		}
	}
	// Vẫn chạm trần sau khi dọn (rất nhiều vân tay khác nhau còn sống) → reset sạch.
	if len(dedupState) >= dedupMaxEntries {
		dedupState = make(map[string]*dedupEntry, 64)
	}
}
