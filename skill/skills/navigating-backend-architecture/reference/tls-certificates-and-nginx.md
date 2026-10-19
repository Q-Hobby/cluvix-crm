# TLS certificate & nginx hosting

Cert wildcard `*.cluvixsolutions.com` là **DV hạn ngắn (~6 tháng)** → gia hạn là việc LẶP LẠI, không phải sự cố một lần.

- [1. Ba nơi phải nạp cert](#1-ba-nơi-phải-nạp-cert)
- [2. Bộ file cert — fullchain vs chain](#2-bộ-file-cert--fullchain-vs-chain)
- [3. Quy trình gia hạn](#3-quy-trình-gia-hạn)
- [4. Verify — 4 phép đo bắt buộc](#4-verify--4-phép-đo-bắt-buộc)
- [5. Gotcha](#5-gotcha)

---

## 1. Ba nơi phải nạp cert

Thiếu 1 nơi = vỡ một mặt trận, và **hết hạn thì cả 3 vỡ cùng lúc** — "đã thay trên server chính" KHÔNG phải là xong.

| Nơi | Phục vụ | Cách nạp |
|---|---|---|
| VPS tenant | 13 vhost `<tenant>.cluvixsolutions.com` | ssh + `sudo`, quy trình mục 3 |
| VPS landing/app | `cluvixsolutions.com`, `www`, `app` | ssh + `sudo`, quy trình mục 3 |
| `cloud.cluvixsolutions.com` — **CDN bên thứ 3** | `S3_CDN`, MỌI ảnh upload | portal nhà cung cấp, **KHÔNG ssh được** → việc của NGƯỜI |

IP, tài khoản ssh, tên nhà cung cấp CDN: ghi ở runbook nội bộ (gitignored) — KHÔNG ghi vào `skill/` vì đây là git repo được theo dõi.

Vhost dùng cert RIÊNG — KHÔNG đụng khi gia hạn wildcard:
`app.customer-f.example`, `customer-d.example`, `vps.customer-d.example` (Cloudflare Origin CA, hạn 2040), `warranty.customer-d.example` (PA Vietnam).


## 2. Bộ file cert — fullchain vs chain

Nginx **không có** directive nạp intermediate riêng — `ssl_certificate` phải là 1 file gộp sẵn.

| File | Nội dung | Directive |
|---|---|---|
| `cluvixsolutions.com_fullchain.crt` | leaf + 2 intermediate | `ssl_certificate` |
| `cluvixsolutions.com_chain.crt` | 2 intermediate (KHÔNG leaf) | `ssl_trusted_certificate` (OCSP stapling) |
| `cluvixsolutions.com_private.key` | private key | `ssl_certificate_key`, `chmod 600` |

Cấu hình dùng chung: `/etc/nginx/snippets/ssl-cluvixsolutions.conf` (mirror ở `host/nginx/snippets/`), mọi vhost chỉ `include snippets/ssl-cluvixsolutions.conf;`.

⚠ `ssl_stapling on` mà thiếu `ssl_trusted_certificate` → stapling **im lặng không chạy**, không báo lỗi.

⚠ Dòng `ssl_ciphers` khai trong vhost **sau** dòng `include` sẽ ĐÈ cipher của snippet — gỡ dòng thừa trong vhost.

## 3. Quy trình gia hạn

```bash
# B1 — sinh fullchain + chain từ bộ file Sectigo giao
cd documents/ssl-cluvixsolutions
{ cat star_*_certificate.crt; printf '\n'; \
  awk '/BEGIN CERT/{n++} n<=2' star_*_rootca.crt; } | grep -v '^$' \
  > cluvixsolutions.com_fullchain.crt
awk '/BEGIN CERT/{n++} n<=2' star_*_rootca.crt > cluvixsolutions.com_chain.crt

# B2 — GATE: key phải khớp cert, 2 giá trị BẮT BUỘC giống nhau
openssl x509 -in cluvixsolutions.com_fullchain.crt -noout -modulus | openssl md5
openssl rsa  -in star_*_privkey.key               -noout -modulus | openssl md5

# B3 — trên MỖI server: backup trước
TS=$(date +%Y%m%d%H%M%S)
sudo mkdir -p /var/www/ssl/backup-$TS
sudo cp -a /var/www/ssl/cluvixsolutions.com_* /var/www/ssl/backup-$TS/
sudo tar czf /root/nginx-sites-backup-$TS.tar.gz -C /etc/nginx sites-available snippets nginx.conf

# B4 — cài (scp lên /tmp trước, /var/www/ssl thuộc root)
sudo install -o root -g root -m 644 /tmp/cluvixsolutions.com_fullchain.crt /var/www/ssl/
sudo install -o root -g root -m 644 /tmp/cluvixsolutions.com_chain.crt     /var/www/ssl/
sudo install -o root -g root -m 600 /tmp/star_cluvixsolutions_com_privkey.key \
     /var/www/ssl/cluvixsolutions.com_private.key

# B5 — GATE cuối, fail thì DỪNG, không reload
sudo nginx -t && sudo systemctl reload nginx
```

Rollback: `sudo cp -a /var/www/ssl/backup-<TS>/* /var/www/ssl/ && sudo nginx -t && sudo systemctl reload nginx`.

## 4. Verify — 4 phép đo bắt buộc

Chạy TỪNG domain, kể cả domain "chắc chắn giống nhau" — vhost có thể trỏ file khác.

```bash
d=<domain>
out=$(echo | openssl s_client -connect 127.0.0.1:443 -servername $d -CApath /etc/ssl/certs 2>/dev/null)
echo "$out" | grep -c "^ *[0-9] s:"          # (1) chain = 3
echo "$out" | grep "Verify return code"      # (2) = 0 (ok)
echo "$out" | openssl x509 -noout -enddate   # (3) hạn mới
echo | openssl s_client -connect 127.0.0.1:443 -servername $d -status 2>/dev/null \
  | grep "OCSP Response Status"              # (4) successful
```

(4) rỗng ở lần gọi ĐẦU là bình thường — nginx nạp OCSP lười; gọi lại vài lần rồi mới kết luận.

## 5. Gotcha

- **File Sectigo giao KHÔNG có newline cuối** (byte cuối = `-`). `cat leaf.crt inter.crt` dính thành `-----END CERTIFICATE----------BEGIN CERTIFICATE-----` → nginx `bad end line`, từ chối nạp. Phải chèn `\n` (B1 ở trên đã xử).
- **Thiếu intermediate KHÔNG lộ qua browser**: Chrome/curl-macOS tự chắp chain qua AIA; Android/`dart:io`/Go/OpenSSL từ chối handshake. Web xanh ≠ cert đúng — luôn đo bằng `openssl s_client` (sự cố MOB-113: vỡ toàn bộ ảnh mobile).
- **`curl` từ localhost trả 403 là geo-block, KHÔNG phải lỗi cert**: `snippets/geo-block.conf` chỉ cho IP Việt Nam, `127.0.0.1` không tra được country. Đối chứng: vhost không bật geo-block (`app.customer-f.example`) trả 200 cùng lúc. Bằng chứng site sống = access log có 2xx từ IP thật.
- **`listen 443 ssl http2;` KHÔNG deprecated trên nginx 1.24.0** (cả 2 server đang 1.24.0) — cú pháp tách `http2 on;` chỉ cần từ 1.25.1. Đừng "sửa" cho hiện đại.
- **`perl -0pi -e '...'`**: `-i` bundle ở cuối nuốt luôn `-e` làm suffix → perl coi script là tên file, báo `Can't open`. Viết tách: `perl -i -0pe '...'`.
- SAN của cert: `*.cluvixsolutions.com` + `cluvixsolutions.com`. Wildcard phủ ĐÚNG 1 cấp — `a.b.cluvixsolutions.com` KHÔNG được phủ.
- Bộ file cert gốc (`documents/ssl-*`) và mirror cấu hình nginx (`host/`) chứa private key ⇒ KHÔNG commit vào repo (repo public); giữ ngoài repo hoặc thêm vào `.gitignore` trước khi đặt vào cây thư mục. Sửa file ở đó: backup ra scratchpad trước, không `mv`/`rm` ngoài phạm vi được giao.
- **`add_header` trong location con XOÁ TOÀN BỘ header cấp server, không merge** (bug thật 2026-09, `app.cluvixsolutions.com.conf`): nginx chỉ kế thừa `add_header` từ block cha khi block con KHÔNG tự khai add_header nào ở cùng level — hễ con có 1 add_header riêng (vd `Cache-Control` cho `*.html`/`*.js`) là mất sạch CSP/HSTS/X-Frame-Options/Referrer-Policy của server{}, im lặng, `nginx -t` vẫn xanh. Fix: tách bộ security header ra file include (`host/nginx/snippets/security-headers.conf`) và `include` lại NGUYÊN VẸN trong MỌI location có add_header riêng, không chỉ ở server{}. Cần 1 location được nhúng cross-origin (vd widget iframe) → tạo snippet thứ 2 không có `X-Frame-Options`/CSP chung, tự khai CSP `frame-ancestors` riêng tại location đó — không set cả `X-Frame-Options: SAMEORIGIN/DENY` lẫn `frame-ancestors *` cùng lúc (mâu thuẫn nhau).
