package logger

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func resetDedup() {
	dedupMu.Lock()
	defer dedupMu.Unlock()
	dedupState = make(map[string]*dedupEntry, 64)
}

// sqlErrMsg dựng message ĐÚNG hình dạng thật của pkg/logger/gorm.go: dòng đầu là lỗi (ổn định),
// các dòng sau là query kèm giá trị KHÁC NHAU mỗi lần.
func sqlErrMsg(id int) string {
	return fmt.Sprintf(
		"SQL Error: Error 1054 (42S22): Unknown column 'is_identity_verified' in 'field list'\n"+
			"Query: INSERT INTO `omi_conversation` (...) VALUES (%d,'zalo_personal',%d)\n"+
			"Rows: 0 | Time: %dµs", id, id*7, id*13)
}

// Ca thật: 100 lỗi/nhịp, nội dung query khác nhau từng tin. Phải ra ĐÚNG 1 tin/nhịp.
func TestDedup_SpamBurstCollapsesToOneMessage(t *testing.T) {
	resetDedup()
	base := time.Date(2026, 9, 4, 11, 0, 0, 0, time.UTC)

	sent := 0
	for i := 0; i < 100; i++ {
		ok, _ := shouldSendAlert(sqlErrMsg(i), base.Add(time.Duration(i)*time.Millisecond))
		if ok {
			sent++
		}
	}
	if sent != 1 {
		t.Fatalf("nhịp 100 lỗi phải gửi đúng 1 tin, thực tế %d", sent)
	}
}

// Nhịp kế tiếp (sau cửa sổ) phải gửi lại VÀ báo số đã nén — không nuốt im lặng.
func TestDedup_NextWindowReportsSuppressedCount(t *testing.T) {
	resetDedup()
	base := time.Date(2026, 9, 4, 11, 0, 0, 0, time.UTC)

	for i := 0; i < 100; i++ {
		shouldSendAlert(sqlErrMsg(i), base.Add(time.Duration(i)*time.Millisecond))
	}

	ok, note := shouldSendAlert(sqlErrMsg(999), base.Add(dedupWindow+time.Second))
	if !ok {
		t.Fatal("cửa sổ mới phải gửi lại")
	}
	if !strings.Contains(note, "99") {
		t.Fatalf("ghi chú phải nêu 99 tin đã nén, thực tế %q", note)
	}
}

// Chống hồi quy quan trọng nhất: nếu vân tay lấy CẢ message thì mỗi tin một vân tay,
// dedup vô tác dụng đúng lúc cần nhất.
func TestDedup_FingerprintIgnoresVaryingQueryBody(t *testing.T) {
	if dedupFingerprint(sqlErrMsg(1)) != dedupFingerprint(sqlErrMsg(2)) {
		t.Fatal("2 lỗi cùng loại nhưng khác giá trị query phải cho CÙNG vân tay")
	}
}

// Lỗi khác loại KHÔNG được che nhau — mỗi loại vẫn phải có tin đầu của mình.
func TestDedup_DistinctErrorsAreIndependent(t *testing.T) {
	resetDedup()
	now := time.Date(2026, 9, 4, 11, 0, 0, 0, time.UTC)

	if ok, _ := shouldSendAlert("SQL Error: lỗi A\nQuery: x", now); !ok {
		t.Fatal("lỗi A phải được gửi")
	}
	if ok, _ := shouldSendAlert("SQL Error: lỗi B\nQuery: y", now); !ok {
		t.Fatal("lỗi B (khác loại) phải được gửi, không bị lỗi A che")
	}
}

func TestDedup_EvictKeepsMapBounded(t *testing.T) {
	resetDedup()
	now := time.Date(2026, 9, 4, 11, 0, 0, 0, time.UTC)

	for i := 0; i < dedupMaxEntries+50; i++ {
		shouldSendAlert(fmt.Sprintf("lỗi loại %d", i), now.Add(time.Duration(i)*3*dedupWindow))
	}
	dedupMu.Lock()
	n := len(dedupState)
	dedupMu.Unlock()
	if n > dedupMaxEntries {
		t.Fatalf("map vượt trần: %d > %d", n, dedupMaxEntries)
	}
}
