# Gotchas Outbound HTTP

## Mục lục

- Outbound HTTP — Force tcp4 khi endpoint fail trên IPv6
- Secret trong URL rò vào log qua error (HIGH)

---


## Outbound HTTP — Force tcp4 khi endpoint fail trên IPv6

**Triệu chứng:** `dial tcp [::1]:443: connect: connection refused` dù URL hợp lệ và endpoint đang chạy.

**Nguyên nhân:** Go mặc định resolve DNS → IPv6 (`::1`) trước IPv4. Một số server/provider (Zalo ZNS, một số internal endpoint) không lắng nghe IPv6 → connection refused.

**Fix:** custom `DialContext` force `tcp4`:
```go
import "net"

dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
transport := &http.Transport{
    DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
        return dialer.DialContext(ctx, "tcp4", addr)
    },
}
client := &http.Client{Transport: transport, Timeout: 10 * time.Second}
```

Áp dụng cho bất kỳ `http.Client` tự tạo gọi external API. (CRM đã gỡ `pkg/httpclient` 2026-10-19 cùng tính năng gợi ý "Có sẵn" lấy JSON từ CDN — không còn outbound tới CDN nội bộ.)

---

## Secret trong URL rò vào log qua error (HIGH)

URL chứa token (Telegram getFile `.../bot<token>/<path>`, Meta `?access_token=`) → transport error `net/http` là `*url.Error`, `.Error()` render NGUYÊN URL (chỉ che userinfo, KHÔNG che path/query) → token vào log khi timeout/DNS/reset. MASK secret khỏi err TRƯỚC khi log: `strings.ReplaceAll(err.Error(), secretURL, "***")`; hoặc mask tại nguồn trong HTTP client. Mask xong trả `errors.New(maskedMsg)` (cắt chain) — wrap `%w` giữ err gốc thì `errors.Unwrap(err).Error()` vẫn lộ token. Precedent: `omnichat` Telegram mirror + Meta client (2026-07-17); `maskGraphErr` config/omni_channel (2026-07-19).

**Hai cách che HỎNG mà test thường không thấy** (đo 2026-08-31, epic omnichat-meta-event-coverage):

1. **Che theo MỘT tên tham số hardcode ⇒ endpoint khác đặt bí mật ở ô khác là không che gì.** `maskGraphErr` chỉ che `access_token`, nhưng `/debug_token` đặt page token của khách ở `input_token` còn `access_token` chỉ là app token ⇒ token khách lộ nguyên văn. Hàm che phải duyệt **danh sách tên tham số bí mật** (`access_token`, `input_token`, `client_secret`, `fb_exchange_token`, `code`…); thêm endpoint mới có tham số bí mật ⇒ thêm tên vào danh sách.
2. **Chỉ `ReplaceAll` bản THÔ ⇒ trượt khi giá trị có ký tự đặc biệt.** App token dạng `{app_id}|{app_secret}` nằm trong URL ở dạng đã escape (`%7C`) nên bản thô không khớp. Phải che cả `url.QueryEscape(v)`.

Cả hai chỉ lộ ra khi có test dựng lỗi transport thật (httptest đóng server ngay) rồi assert chuỗi bí mật KHÔNG có trong `err.Error()`.
