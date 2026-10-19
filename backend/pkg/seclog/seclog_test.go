package seclog

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
)

// TestSinkDeliversEntries — Event đẩy Entry vào sink; batch flush nhận đủ, promote
// đúng user_id/username sang cột và đẩy field còn lại vào Detail. Story-40 (D15): Entry bỏ
// CompanyID + helper seclog.CompanyID — trước đây assert company_id = 7 được promote sang cột.
func TestSinkDeliversEntries(t *testing.T) {
	var mu sync.Mutex
	got := make([]Entry, 0, 64)
	done := make(chan struct{})

	InitSink(1000, func(batch []Entry) {
		mu.Lock()
		got = append(got, batch...)
		n := len(got)
		mu.Unlock()
		if n >= 64 {
			select {
			case <-done:
			default:
				close(done)
			}
		}
	})

	// 64 event → chạm batchMax → flush ngay (không đợi ticker 2s).
	for i := 0; i < 64; i++ {
		Event(nil, "login_success",
			UserID(uint(i+1)),
			Username("bob"),
			FP("abcdef0123456789"),
			zap.Bool("enforced", true),
		)
	}

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("sink did not flush 64 entries in time")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(got) < 64 {
		t.Fatalf("got %d entries, want >= 64", len(got))
	}
	e := got[0]
	if e.Event != "login_success" {
		t.Fatalf("event = %q", e.Event)
	}
	if e.UserID == nil || *e.UserID != 1 {
		t.Fatalf("user_id = %v, want 1", e.UserID)
	}
	if e.Username != "bob" {
		t.Fatalf("username = %q", e.Username)
	}
	// fp bị FP() cắt còn 8 ký tự; enforced vào Detail.
	if e.Detail["fp"] != "abcdef01" {
		t.Fatalf("detail.fp = %v, want abcdef01", e.Detail["fp"])
	}
	if e.Detail["enforced"] != true {
		t.Fatalf("detail.enforced = %v, want true", e.Detail["enforced"])
	}
}

// TestEventNilLoggerNoPanic — Event không panic khi logger.L nil (đã guard).
func TestEventNilLoggerNoPanic(t *testing.T) {
	// Không InitSink → sinkCh nil; logger.L nil trong test. Chỉ cần không panic.
	Event(nil, "access_ip_mismatch")
}

// TestEventCatTenQuaDai — event dài hơn cột DB bị CẮT tại Event(), không đẩy nguyên xuống.
//
// Assert số TUYỆT ĐỐI (64), KHÔNG so lại MaxEventLen: so với chính hằng số đang kiểm thì đổi
// hằng số kiểu gì test cũng xanh, mà cột DB thì không tự nới theo.
func TestEventCatTenQuaDai(t *testing.T) {
	var got []Entry
	InitSink(16, func(b []Entry) { got = append(got, b...) })
	defer func() { sinkCh = nil }()

	long := strings.Repeat("x", 100)
	Event(nil, long)
	Event(nil, "login_success")

	deadline := time.Now().Add(5 * time.Second)
	for len(got) < 2 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if len(got) < 2 {
		t.Fatalf("sink nhận %d entry, cần 2", len(got))
	}
	if len(got[0].Event) != 64 {
		t.Fatalf("event dài %d, phải bị cắt còn 64", len(got[0].Event))
	}
	// Entry đứng sau PHẢI còn nguyên — đây mới là thứ lỗi 1406 từng đánh rơi cả lô.
	if got[1].Event != "login_success" {
		t.Fatalf("entry sau = %q, phải nguyên vẹn", got[1].Event)
	}
}

// TestKhongTenEventNaoVuotCotDB — quét TOÀN repo: mọi seclog.Event("...") phải vừa cột VARCHAR(64).
// Cắt chuỗi là lưới cuối; tên vượt cột vẫn là lỗi cần sửa tại chỗ gọi (log cắt cụt khó điều tra).
func TestKhongTenEventNaoVuotCotDB(t *testing.T) {
	root := filepath.Join("..", "..")
	re := regexp.MustCompile(`seclog\.Event\([^,]+,\s*"([a-z0-9_]+)"`)
	var bad []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		b, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		for _, m := range re.FindAllStringSubmatch(string(b), -1) {
			if len(m[1]) > 64 {
				bad = append(bad, m[1]+" ("+path+")")
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(bad) > 0 {
		t.Fatalf("event name vượt 64 ký tự:\n  %s", strings.Join(bad, "\n  "))
	}
}
