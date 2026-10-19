package helpers

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

type stubResolver struct{ ips []net.IP }

func (s stubResolver) LookupIP(_ context.Context, _, _ string) ([]net.IP, error) {
	return s.ips, nil
}

// TestSafeMediaFetch_BlockList — Lớp 2: host là IP literal riêng tư/loopback/link-local/metadata
// (kể cả IPv4-mapped IPv6) → ErrSSRFBlocked TRƯỚC mọi network I/O. Không allow-list.
func TestSafeMediaFetch_BlockList(t *testing.T) {
	blocked := []string{
		"http://127.0.0.1/x",
		"http://10.0.0.5/x",
		"http://192.168.1.10/x",
		"http://172.16.0.1/x",
		"http://169.254.169.254/latest/meta-data/", // cloud metadata — vector SSRF kinh điển
		"http://0.0.0.0/x",
		"http://100.100.0.1/x",       // 100.64/10 CGNAT
		"http://[::1]/x",             // IPv6 loopback
		"http://[fd00::1]/x",         // IPv6 ULA
		"http://[fe80::1]/x",         // IPv6 link-local
		"http://[::ffff:127.0.0.1]/", // IPv4-mapped loopback (bypass phổ biến)
		"http://[::ffff:169.254.169.254]/",
		"file:///etc/passwd", // scheme không hợp lệ
		"gopher://127.0.0.1/", // scheme không hợp lệ
	}
	for _, raw := range blocked {
		resp, err := SafeMediaFetch(context.Background(), raw, nil)
		if err == nil {
			if resp != nil {
				resp.Body.Close()
			}
			t.Fatalf("URL %q phải bị chặn nhưng lại pass", raw)
		}
	}
}

// TestSafeMediaFetch_AllowList — Lớp 1: host trong allow-list được fetch thẳng, KỂ CẢ khi resolve
// ra loopback (block-list bị bỏ qua vì allow-list là primary control). httptest chạy trên 127.0.0.1.
func TestSafeMediaFetch_AllowList(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("IMG-BYTES"))
	}))
	defer ts.Close()

	u, _ := url.Parse(ts.URL)
	host := u.Hostname() // "127.0.0.1"

	// Không allow-list → phải bị chặn (loopback).
	if resp, err := SafeMediaFetch(context.Background(), ts.URL, nil); err == nil {
		if resp != nil {
			resp.Body.Close()
		}
		t.Fatalf("loopback không allow-list phải bị chặn")
	}

	// Có allow-list host → pass.
	resp, err := SafeMediaFetch(context.Background(), ts.URL, []string{host})
	if err != nil {
		t.Fatalf("allow-list host phải fetch được: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "IMG-BYTES" {
		t.Fatalf("body sai: %q", body)
	}
}

// TestSafeMediaFetch_PinResolve — Lớp 3: host KHÔNG allow-list → resolve qua resolver → validate →
// PIN dial đúng IP đã resolve. Stub resolver map "media.test" → IP httptest; isBlocked cho phép
// loopback để chứng minh đường pin CONNECT tới đúng IP resolve.
func TestSafeMediaFetch_PinResolve(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("PINNED-OK"))
	}))
	defer ts.Close()

	u, _ := url.Parse(ts.URL)
	srvIP := net.ParseIP(u.Hostname())
	if srvIP == nil {
		t.Fatalf("không parse được IP httptest: %s", u.Hostname())
	}

	f := &mediaFetcher{
		resolver:  stubResolver{ips: []net.IP{srvIP}},
		isBlocked: func(net.IP) bool { return false }, // cho phép loopback trong test pin
		timeout:   5 * time.Second,
	}
	resp, err := f.fetch(context.Background(), "http://media.test:"+u.Port()+"/img.jpg")
	if err != nil {
		t.Fatalf("pin fetch phải thành công (dial IP đã resolve): %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "PINNED-OK" {
		t.Fatalf("body sai: %q", body)
	}
}

// TestSafeMediaFetch_PinRejectsRebind — cùng stub resolver (map host lạ → loopback) nhưng isBlocked
// THẬT (chặn loopback) → validate tại dial phát hiện IP private → ErrSSRFBlocked. Đây là lá chắn
// DNS-rebinding: dù host "sạch" lúc parse, IP resolve ra private vẫn bị chặn tại connect.
func TestSafeMediaFetch_PinRejectsRebind(t *testing.T) {
	f := &mediaFetcher{
		resolver:  stubResolver{ips: []net.IP{net.ParseIP("127.0.0.1")}},
		isBlocked: isBlockedIP, // block-list thật
		timeout:   5 * time.Second,
	}
	resp, err := f.fetch(context.Background(), "http://media.test/img.jpg")
	if err == nil {
		if resp != nil {
			resp.Body.Close()
		}
		t.Fatalf("host resolve ra loopback phải bị chặn (rebind guard)")
	}
}

func TestIsAllowedMediaHost(t *testing.T) {
	allowed := []string{"zinst-stc.zadn.vn", ".zadn.vn", ".zalo.me"}
	cases := map[string]bool{
		"zinst-stc.zadn.vn": true,
		"f13-zpg.zadn.vn":   true,  // suffix .zadn.vn
		"zadn.vn":           true,  // suffix trần
		"cdn.zalo.me":       true,  // suffix .zalo.me
		"evil.com":          false,
		"zadn.vn.evil.com":  false, // KHÔNG match .zadn.vn (kết thúc .evil.com)
		"notzadn.vn":        false, // ".zadn.vn" yêu cầu dấu chấm phân tách
	}
	for host, want := range cases {
		if got := IsAllowedMediaHost(host, allowed); got != want {
			t.Fatalf("IsAllowedMediaHost(%q) = %v, muốn %v", host, got, want)
		}
	}
}

func TestIsBlockedIP(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "10.1.2.3", "192.168.0.1", "172.20.0.1", "169.254.169.254",
		"0.0.0.0", "100.64.0.1", "::1", "fd12::1", "fe80::1", "::ffff:10.0.0.1",
	}
	for _, s := range blocked {
		if !isBlockedIP(net.ParseIP(s)) {
			t.Fatalf("%s phải bị chặn", s)
		}
	}
	public := []string{"8.8.8.8", "1.1.1.1", "203.0.113.5", "2001:4860:4860::8888"}
	for _, s := range public {
		if isBlockedIP(net.ParseIP(s)) {
			t.Fatalf("%s là public, KHÔNG được chặn", s)
		}
	}
}
