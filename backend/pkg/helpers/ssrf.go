package helpers

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SSRF guard cho việc tải media từ URL do webhook/khách cung cấp (attacker-influenced).
//
// URL attachment nằm trong payload webhook Zalo/Meta/... nội dung do người ngoài tạo — 1 actor
// có thể nhét URL trỏ vào 169.254.169.254 (cloud metadata) hoặc 127.0.0.1:6379 (Redis nội bộ),
// biến server thành SSRF proxy. Vì vậy MỌI lần tải media phải qua SafeMediaFetch, KHÔNG http.Get trần.
//
// 3 lớp chặn (port từ ZCRM-EE ssrf-guard.ts — skill/bmad/omnichannel/review-zcrm/03-media-pipeline.md §2):
//   1. Allow-list hostname (primary control): host thuộc CDN đã biết trước (Zalo) → tin cậy, fetch thẳng.
//   2. Block-list literal IP: host là IP literal riêng tư/loopback/link-local/metadata (kể cả
//      IPv4-mapped IPv6 ::ffff:127.0.0.1) → chặn ngay, không resolve.
//   3. DNS resolve → validate MỌI IP → PIN IP tại connect (DialContext resolve-và-connect CÙNG 1 lần):
//      đóng lỗ DNS-rebinding TOCTOU (validate IP public rồi client tự resolve lại ra IP private lúc dial).

var (
	// ErrSSRFBlocked — host/IP đích bị chặn (riêng tư/loopback/metadata) hoặc scheme không hợp lệ.
	ErrSSRFBlocked = errors.New("ssrf: destination host/IP blocked")
	// ErrSSRFNoIP — DNS resolve không ra IP nào.
	ErrSSRFNoIP = errors.New("ssrf: no IP resolved for host")
)

// mediaFetchTimeout — timeout tổng 1 lần tải (kể cả dial+read). CDN chậm không treo goroutine mãi.
const mediaFetchTimeout = 20 * time.Second

// mediaMaxRedirects — số redirect tối đa cho phép (mỗi hop vẫn qua DialContext validate IP).
const mediaMaxRedirects = 5

// ipResolver — abstraction cho DNS lookup (net.DefaultResolver production; stub trong test pin).
type ipResolver interface {
	LookupIP(ctx context.Context, network, host string) ([]net.IP, error)
}

// SafeMediaFetch tải rawURL qua guard 3 lớp. allowedHosts = allow-list hostname per kênh
// (exact "zinst-stc.zadn.vn" hoặc suffix ".zadn.vn"); caller (wire) truyền vào — xem
// omnichat/service RegisterMediaHosts. Trả *http.Response (caller PHẢI đóng Body) hoặc lỗi.
func SafeMediaFetch(ctx context.Context, rawURL string, allowedHosts []string) (*http.Response, error) {
	return newMediaFetcher(allowedHosts).fetch(ctx, rawURL)
}

// IsAllowedMediaHost — export cho caller log/quyết định (host có nằm allow-list không). host so
// case-insensitive; entry bắt đầu bằng "." là suffix match.
func IsAllowedMediaHost(host string, allowedHosts []string) bool {
	return isAllowedMediaHost(host, allowedHosts)
}

// SafeOutboundClient trả *http.Client chặn SSRF cho request tới URL do USER tự nhập (webhook outbound,
// story-04): KHÔNG host-allowlist (mọi host công khai đều hợp lệ) — CHỈ chặn IP nội bộ (private/loopback/
// link-local/metadata) qua guardedDial pin-IP mỗi hop (resolve+validate+dial cùng 1 lần → không TOCTOU).
// Tái dùng guardedDial/isBlockedIP sẵn có (KHÔNG viết lại). timeout áp cho toàn request.
//
// LƯU Ý test: guardedDial chặn 127.0.0.1 nên httptest.Server (loopback) KHÔNG gửi qua client này — happy
// path test inject client riêng; client này dùng để verify block IP nội bộ + production.
func SafeOutboundClient(timeout time.Duration) *http.Client {
	f := &mediaFetcher{
		resolver:  net.DefaultResolver,
		isBlocked: isBlockedIP,
		timeout:   timeout,
	}
	transport := &http.Transport{
		Proxy:                 nil, // KHÔNG proxy env — tránh proxy nội bộ vô hiệu hoá guard
		DialContext:           f.guardedDial,
		TLSHandshakeTimeout:   timeout,
		ResponseHeaderTimeout: timeout,
		ExpectContinueTimeout: 1 * time.Second,
		DisableKeepAlives:     true,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= mediaMaxRedirects {
				return errors.New("ssrf: too many redirects")
			}
			// Redirect tới host mới vẫn qua guardedDial (validate IP); chỉ chặn scheme lạ.
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return ErrSSRFBlocked
			}
			return nil
		},
	}
}

type mediaFetcher struct {
	allowedHosts []string
	resolver     ipResolver
	isBlocked    func(net.IP) bool
	timeout      time.Duration
}

func newMediaFetcher(allowedHosts []string) *mediaFetcher {
	return &mediaFetcher{
		allowedHosts: allowedHosts,
		resolver:     net.DefaultResolver,
		isBlocked:    isBlockedIP,
		timeout:      mediaFetchTimeout,
	}
}

func (f *mediaFetcher) fetch(ctx context.Context, rawURL string) (*http.Response, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, ErrSSRFBlocked // chặn file://, gopher://, dict://... vector SSRF khác
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return nil, ErrSSRFBlocked
	}

	// Lớp 1 — allow-list: CDN tin cậy đã biết trước → fetch thẳng (không cần pin, đỡ overhead).
	// (QA follow-up 2026-07-17: cân nhắc luôn guardedDial cả host allow-list để chặn open-redirect
	// trên chính CDN tin cậy — low-likelihood, cần rework test seam httptest/loopback trong story
	// hardening riêng, KHÔNG nhồi vội ở đây.)
	if isAllowedMediaHost(host, f.allowedHosts) {
		return f.do(ctx, rawURL, false)
	}

	// Lớp 2 — block-list literal: host là IP literal riêng tư → chặn ngay (không resolve).
	if ip := net.ParseIP(host); ip != nil {
		if f.isBlocked(ip) {
			return nil, ErrSSRFBlocked
		}
	} else {
		// Lớp 3 (pre-flight) — resolve + validate MỌI IP. Bất kỳ IP nào bị chặn → từ chối cả host
		// (không cherry-pick IP "sạch" trong tập trả về — 1 IP xấu là đủ để nghi ngờ rebind).
		ips, rerr := f.resolver.LookupIP(ctx, "ip", host)
		if rerr != nil {
			return nil, rerr
		}
		if len(ips) == 0 {
			return nil, ErrSSRFNoIP
		}
		for _, ip := range ips {
			if f.isBlocked(ip) {
				return nil, ErrSSRFBlocked
			}
		}
	}

	// Lớp 3 (connect) — pin: DialContext tự resolve+validate+dial CÙNG 1 IP → không có khoảng hở
	// giữa check và connect (DNS-rebinding). Áp cho mọi hop redirect.
	return f.do(ctx, rawURL, true)
}

// do thực hiện HTTP GET. guardDial=true → gắn DialContext validate+pin IP mỗi lần dial; =false →
// dùng dialer mặc định (host đã allow-list, tin cậy).
func (f *mediaFetcher) do(ctx context.Context, rawURL string, guardDial bool) (*http.Response, error) {
	transport := &http.Transport{
		Proxy:                 nil, // KHÔNG dùng proxy env — tránh proxy nội bộ vô hiệu hoá guard
		TLSHandshakeTimeout:   f.timeout,
		ResponseHeaderTimeout: f.timeout,
		ExpectContinueTimeout: 1 * time.Second,
		DisableKeepAlives:     true,
	}
	if guardDial {
		transport.DialContext = f.guardedDial
	} else {
		transport.DialContext = (&net.Dialer{Timeout: f.timeout}).DialContext
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   f.timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= mediaMaxRedirects {
				return errors.New("ssrf: too many redirects")
			}
			// Redirect tới host mới vẫn phải qua DialContext (validate IP) — chỉ cần chặn scheme lạ.
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return ErrSSRFBlocked
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

// guardedDial — resolve host của addr, validate MỌI IP, rồi dial ĐÚNG IP vừa validate (pin). Resolve
// và connect trong CÙNG lời gọi nên không có TOCTOU. Áp cho cả request gốc lẫn mọi redirect.
func (f *mediaFetcher) guardedDial(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	var ips []net.IP
	if ip := net.ParseIP(host); ip != nil {
		ips = []net.IP{ip}
	} else {
		ips, err = f.resolver.LookupIP(ctx, "ip", host)
		if err != nil {
			return nil, err
		}
	}
	if len(ips) == 0 {
		return nil, ErrSSRFNoIP
	}
	for _, ip := range ips {
		if f.isBlocked(ip) {
			return nil, ErrSSRFBlocked
		}
	}
	// Dial IP đầu tiên đã validate (pin) — KHÔNG để http.Client tự resolve lại.
	dialer := &net.Dialer{Timeout: f.timeout}
	return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
}

// isBlockedIP — true nếu ip thuộc dải riêng tư/loopback/link-local/metadata/multicast/"this network".
// Dùng method chuẩn của net.IP (tự xử lý dạng IPv4-mapped IPv6 ::ffff:x qua To4()) + vài dải bổ sung.
func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || // 127.0.0.0/8, ::1
		ip.IsPrivate() || // 10/8, 172.16/12, 192.168/16, fc00::/7 (ULA)
		ip.IsUnspecified() || // 0.0.0.0, ::
		ip.IsLinkLocalUnicast() || // 169.254/16 (gồm metadata 169.254.169.254), fe80::/10
		ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast() {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		// 0.0.0.0/8 "this network" (IsUnspecified chỉ bắt đúng 0.0.0.0).
		if v4[0] == 0 {
			return true
		}
		// 100.64.0.0/10 — Carrier-Grade NAT (RFC6598), có thể route tới hạ tầng nội bộ.
		if v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
			return true
		}
	}
	return false
}

// isAllowedMediaHost — host khớp allow-list. Entry "exact.host" so bằng; entry ".suffix" khớp
// mọi subdomain kết thúc bằng suffix (và cả chính "suffix" trần).
func isAllowedMediaHost(host string, allowed []string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if host == "" {
		return false
	}
	for _, a := range allowed {
		a = strings.ToLower(strings.TrimSpace(a))
		if a == "" {
			continue
		}
		if strings.HasPrefix(a, ".") {
			if host == a[1:] || strings.HasSuffix(host, a) {
				return true
			}
			continue
		}
		if host == a {
			return true
		}
	}
	return false
}
