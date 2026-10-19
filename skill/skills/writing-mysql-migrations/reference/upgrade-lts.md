# Upgrade MySQL LTS — quá trình

## Mục lục

- Đường đi bắt buộc: logical dump, KHÔNG in-place
- A. Chuẩn bị (bản cũ còn chạy)
- B. Dump + verify
- C. Restore-test vào version đích — gate không được bỏ
- D. Gỡ bản cũ
- E. Cài bản mới
- F. Dựng user + import
- G. Dọn
- Bẫy im lặng — tổng hợp

---


> Áp dụng cho mọi server Cluvix (prod + dev). Đã chạy thật: 8.0.46 → 9.7.2 trên Ubuntu 24.04 noble.

## Đường đi bắt buộc: logical dump, KHÔNG in-place

- Docs MySQL **cấm nhảy cóc LTS khi upgrade in-place** — 8.0 → 9.7 phải qua 8.4 LTS.
- Dump logic + cài mới **không** phải in-place (không đụng data dictionary cũ) → né được ràng buộc, đi thẳng 1 nhịp.
- Đánh đổi: mất bước 8.4 trung gian vốn báo incompatibility. Bù bằng upgrade checker nhắm thẳng version đích **trước khi dump**:

```bash
mysqlsh -u root -p -e "util.checkForServerUpgrade({targetVersion:'9.7.0'})"
```

---

## A. Chuẩn bị (bản cũ còn chạy)

```bash
# đổi plugin auth — 9.x KHÔNG còn mysql_native_password (xem mysql97.md §6)
mysql -u root -p -e "SELECT user,host,plugin FROM mysql.user WHERE plugin='mysql_native_password';"
mysql -u root -p -e "ALTER USER 'x'@'localhost' IDENTIFIED WITH caching_sha2_password BY '<pass cũ>';"

# lưu grants — mất file này thì phải dựng lại user từ .env của app
mysql -u root -p -N -e "SELECT CONCAT('SHOW GRANTS FOR ''',user,'''@''',host,''';')
  FROM mysql.user WHERE user NOT LIKE 'mysql.%';" | mysql -u root -p -N > grants.txt

sudo cp -a /etc/mysql etc-mysql-old-backup
```

Giữ nguyên mật khẩu cũ khi `ALTER USER` → không phải sửa `.env` của app.

## B. Dump + verify

`backend/scripts/db-backup-all.sh` — dump từng DB `cluvix_*`, có sẵn 3 gate.

- Dump **từng DB riêng**, KHÔNG `--all-databases`.
- Cờ bắt buộc: `--single-transaction --quick --routines --events`.
  - Thiếu `--single-transaction` → mysqldump **khoá bảng**, prod kẹt ghi.
  - Thiếu `--routines --events` → stored procedure/function/event **mất im lặng**, file vẫn trọn vẹn.
- Gate: `tail -1` phải là `-- Dump completed on`; số `^CREATE TABLE` phải khớp `information_schema.tables`.
- Copy dump **ra khỏi máy** — dump nằm trên chính máy sắp gỡ MySQL không tính là backup.

## C. Restore-test vào version đích — gate không được bỏ

Bước **duy nhất** chứng minh dump dùng được, chứ không chỉ "trông đầy đủ". Import vào instance sạch đúng version đích (docker hoặc máy local), đếm bảng, rồi drop.

```bash
docker run -d --name mytest -e MYSQL_ROOT_PASSWORD=test mysql:9.7
```

**Chưa đủ 100% PASS thì tuyệt đối không sang bước D.**

## D. Gỡ bản cũ

```bash
sudo systemctl stop mysql && sudo systemctl disable mysql
sudo tar -cf mysql-datadir-old.tar -C /var/lib mysql     # rollback cuối

sudo apt remove -y mysql-server mysql-server-8.0 mysql-client-8.0 \
                   mysql-client-core-8.0 mysql-server-core-8.0
sudo apt autoremove -y
```

- ⛔ **KHÔNG `--purge`** — purge của `mysql-server-8.0` chạy debconf rồi dọn luôn data files. Đây là thứ đã thổi bay `mysql.ibd` trên server dev 2026-08-13.
- Đổi tên, KHÔNG xoá — giữ đường lùi:

```bash
sudo mv /var/lib/mysql /var/lib/mysql-8.0-old
sudo mv /etc/mysql     /etc/mysql-8.0-old
```

Cả hai đường dẫn phải **không còn tồn tại** trước `apt install`:
- Còn `/var/lib/mysql` → gói mới thấy datadir có sẵn, không initialize → biến thành in-place upgrade trái phép, mysqld chết lúc nâng data dictionary.
- Còn `/etc/mysql` → mysqld không start; config 8.0 chứa option đã gỡ ở 8.4+ (`default_authentication_plugin`, `expire_logs_days`, `master_info_repository`, các biến `slave_*` → `replica_*`).

## E. Cài bản mới

```bash
wget https://dev.mysql.com/get/mysql-apt-config_<mới nhất>_all.deb
md5sum mysql-apt-config_*.deb        # đối chiếu với trang dev.mysql.com/downloads/repo/apt/
sudo dpkg -i mysql-apt-config_*.deb  # menu → MySQL Server & Cluster → chọn mysql-9.7-lts

sudo apt update
apt-cache policy mysql-server        # GATE: Candidate: phải là 9.7.x
sudo apt install -y mysql-server
```

- Version của `mysql-apt-config` **không liên quan** version MySQL — nó chỉ là gói cấu hình repo. Bản quá cũ **không có kênh LTS mới** trong menu → cài nhầm bản cũ mà không nhận ra. Luôn tải bản mới nhất từ trang download.
- Menu có cả kênh calendar-versioned (`26.x`, Innovation) — chọn đúng dòng `-lts`.
- Debconf hỏi authentication plugin → chọn **Strong Password Encryption** (`caching_sha2_password`).
- Root để trống mật khẩu → `auth_socket`, chỉ vào được bằng `sudo mysql`.

## F. Dựng user + import

`backend/scripts/db-restore-all.sh`.

```sql
CREATE USER 'cluvix'@'localhost' IDENTIFIED WITH caching_sha2_password BY '<pass trong .env>';
GRANT ALL PRIVILEGES ON `cluvix\_%`.* TO 'cluvix'@'localhost';
```

- **`\_` phải escape** trong pattern của GRANT — `_` không escape là wildcard "một ký tự bất kỳ".
- Grant wildcard này gồm cả `CREATE DATABASE` cho tên khớp → khách mới (DB-per-customer) không phải grant lại.
- **Import bằng user khớp `DEFINER` trong dump** (`grep -o "DEFINER=[^ ]*" *.sql`). Import bằng root → view mang definer là user không tồn tại; import bằng user thường mà definer khác → `ERROR 1227` vì thiếu `SET_USER_ID`.
- **`@'localhost'` (socket) ≠ `@'127.0.0.1'` (TCP)** — phải khớp `DB_HOST` trong `.env`, sai là `Access denied` dù pass đúng.
- **KHÔNG restore DB `mysql` / `sys` từ dump bản cũ** — kéo `mysql_native_password` quay lại.
- `FLUSH PRIVILEGES` không cần sau `GRANT`/`CREATE USER`; user không có quyền `RELOAD` gọi nó sẽ `Access denied`.

## G. Dọn

Chỉ xoá `/var/lib/mysql-8.0-old`, `/etc/mysql-8.0-old`, file `.tar` **sau khi app chạy ổn vài ngày**.

---

## Bẫy im lặng — tổng hợp

| Bẫy | Hậu quả | Chặn bằng |
|---|---|---|
| `--purge` thay vì `remove` | Mất sạch data files | Dùng `apt remove` |
| Dump đứt giữa chừng | Backup vô dụng, phát hiện sau khi đã gỡ | Gate `-- Dump completed on` |
| Thiếu `--routines --events` | Procedure/event biến mất, file vẫn hợp lệ | Cờ mặc định trong script |
| `/var/lib/mysql` còn lại | In-place upgrade trái phép | `ls` trước `apt install` |
| `/etc/mysql` còn lại | mysqld không start, log khó đoán | `mv` sang `*-old` |
| `mysql-apt-config` cũ | Menu không có kênh LTS đích | Tải bản mới nhất |
| GRANT `cluvix_%` không escape | Pattern rộng hơn ý định | `` `cluvix\_%` `` |
