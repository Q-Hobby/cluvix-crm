# MySQL 9.7 LTS — syntax & tính năng

## Mục lục

- 1. KHÔNG đổi — vẫn phải PREPARE/EXECUTE cho migration idempotent
- 2. Inline + implicit FOREIGN KEY — nay tạo FK THẬT (đổi hành vi so với 8.0)
- 3. Functional UNIQUE index — thay được cột GENERATED STORED
- 4. Hypergraph optimizer — nay có trong Community
- 5. `EXPLAIN ANALYZE FORMAT=JSON INTO @var` — lưu plan để đối chiếu
- 6. Authentication — `mysql_native_password` đã bị XÓA
- 7. Thay đổi hành vi 9.7.0 cần soi khi upgrade
- 8. Có nhưng CHƯA dùng trong dự án
- 9. Lẫn collation — ĐÃ VÁ trên dev (2026-08-13), CHƯA vá trên DB khách
- 10. Chưa làm — BẮT BUỘC trước khi apply lên DB khách
- Nguồn

---


> **Đọc file này trước khi viết migration hoặc tối ưu query nặng.**
> Mục tiêu: dùng đúng khả năng của server đang chạy, không viết theo giả định MySQL 8.0.

Phiên bản đang chạy: local **9.7.1** (Homebrew, macOS arm64) · dev + prod **9.7.2** (Ubuntu 24.04 noble, repo `mysql-9.7-lts`).
LTS đầu tiên sau 8.4, gom toàn bộ innovation 9.0→9.6. Quy trình upgrade: `upgrade-lts.md`.
Client/driver: `go-sql-driver/mysql v1.10.0` + `gorm.io/driver/mysql v1.6.0`.
Kiểm phiên bản: `/opt/homebrew/bin/mysql -u root -p<pass> -e "SELECT VERSION()"`.

Mọi mục đánh dấu ✅ **verified** = đã probe thật trên 9.7.1 local (DB `*_tmp` dùng-một-lần, đã drop).

---

## 1. KHÔNG đổi — vẫn phải PREPARE/EXECUTE cho migration idempotent

✅ verified: 9.7.1 vẫn báo `ERROR 1064` với mọi dạng dưới đây.

| Cú pháp | 9.7.1 |
|---|---|
| `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` | ❌ 1064 |
| `ALTER TABLE ... DROP COLUMN IF EXISTS` | ❌ 1064 |
| `ALTER TABLE ... ADD INDEX IF NOT EXISTS` | ❌ 1064 |
| `ALTER TABLE ... DROP INDEX IF EXISTS` | ❌ 1064 |
| `ALTER TABLE ... ADD CONSTRAINT IF NOT EXISTS` | ❌ 1064 |
| `CREATE INDEX IF NOT EXISTS` (standalone) | ❌ 1064 |

→ Pattern `SET @sql = IF(<information_schema>...) + PREPARE/EXECUTE` trong `skills/writing-mysql-migrations/SKILL.md` **giữ nguyên 100%**. Không viết migration dựa trên tin đồn "MySQL 9 đã có IF NOT EXISTS".

---

## 2. Inline + implicit FOREIGN KEY — nay tạo FK THẬT (đổi hành vi so với 8.0)

8.0 parse rồi **im lặng bỏ qua** `REFERENCES` mức cột. 9.6+ **tạo và enforce** FK thật.

✅ verified trên 9.7.1:
```sql
CREATE TABLE child_inline (id int unsigned PRIMARY KEY,
  parent_id int unsigned REFERENCES parent(id)) ENGINE=InnoDB;
-- → sinh CONSTRAINT `child_inline_ibfk_1` FOREIGN KEY (parent_id) REFERENCES parent(id)
INSERT INTO child_inline VALUES (1, 999);  -- → ERROR 1452, FK enforce thật

-- implicit: bỏ luôn cột đích, tự trỏ PRIMARY KEY bảng cha
CREATE TABLE child_implicit (id int unsigned PRIMARY KEY, parent_id int unsigned,
  FOREIGN KEY (parent_id) REFERENCES parent) ENGINE=InnoDB;
```

**RULE — vẫn khai `CONSTRAINT` tường minh, KHÔNG dùng inline/implicit:**
- Tên auto `<table>_ibfk_N` phá convention `fk-<table>-<col>` của dự án.
- Guard idempotent tra `INFORMATION_SCHEMA.TABLE_CONSTRAINTS ... CONSTRAINT_NAME='fk-...'` → tên auto = guard không bao giờ match → migration re-run cố ADD lại → fail.
- Tên `_ibfk_N` phụ thuộc thứ tự tạo → không ổn định để `DROP FOREIGN KEY` sau này.

**Rủi ro khi upgrade từ 8.0:** migration cũ có inline `REFERENCES` (8.0 bỏ qua) chạy lại trên 9.7 sẽ tạo FK thật → fail nếu data có orphan. ✅ đã scan `backend/migrations/*.sql`: **0 chỗ** dùng inline REFERENCES → dự án không dính.

---

## 3. Functional UNIQUE index — thay được cột GENERATED STORED

✅ verified: `UNIQUE KEY uq ((IF(st=1,k,NULL)))` tạo được trực tiếp, không cần cột generated.

```sql
-- Pattern "chỉ 1 row active per key" — bản gọn, không thêm cột
CREATE TABLE t (id int PRIMARY KEY, k varchar(20), st tinyint,
  UNIQUE KEY uq ((IF(st = 1, k, NULL)))) ENGINE=InnoDB;
```

- Dùng cho **bảng mới**. Bảng đã có cột `active_key` GENERATED STORED (`approval_instance`, `zns_recipient`) — **giữ nguyên**, không migrate lại.
- Cột trong biểu thức phải nullable-safe; `NULL ≠ NULL` nên nhiều row inactive không đụng nhau (giống pattern cũ).
- Vẫn KHÔNG map biểu thức vào struct Go.
- Trước khi ADD trên bảng có data: dedupe trước (xem `skills/writing-mysql-migrations/SKILL.md` — "Đổi unique key trên bảng có dữ liệu").
- ⛔ Nếu buộc phải dùng CỘT generated (bảng cũ) và cột nền còn cần **FOREIGN KEY**: `STORED` bị MySQL
  từ chối — `Error 3192: Cannot add foreign key on the base column of stored column` (đo thật). Dùng
  `VIRTUAL`: vẫn đánh được UNIQUE index và vẫn cho FK trên cột nền. Bảng MỚI thì dùng functional
  index ở trên, không cần cột nào nên không gặp ràng buộc này.

---

## 4. Hypergraph optimizer — nay có trong Community

Tắt mặc định. Bật session-scope để so plan cho query join nhiều bảng (report doanh thu, payment, treatment).

```sql
SET SESSION optimizer_switch='hypergraph_optimizer=on';
EXPLAIN FORMAT=TREE SELECT ...;   -- chỉ FORMAT=TREE
SET SESSION optimizer_switch='hypergraph_optimizer=off';
```

✅ verified: bật/tắt được, `EXPLAIN FORMAT=TREE` ra plan hash-join bình thường.

**Giới hạn:**
- Bỏ qua MỌI optimizer hint trừ `STRAIGHT_JOIN`.
- KHÔNG hỗ trợ `EXPLAIN FORMAT=TRADITIONAL` và `FORMAT=JSON` → mọi tool/monitor đọc EXPLAIN JSON sẽ hỏng.
- Đổi plan có thể làm query đang nhanh chậm đi.

**RULE:**
- **CẤM `SET GLOBAL`** và cấm bật trong `my.cnf` / code Go.
- Chỉ dùng làm công cụ **chẩn đoán thủ công** khi tối ưu 1 query cụ thể; nếu plan hypergraph tốt hơn → sửa query/index để optimizer mặc định cũng chọn được, KHÔNG ship code phụ thuộc hypergraph.
- Muốn thử 1 câu lẻ: `SELECT /*+ SET_VAR(optimizer_switch='hypergraph_optimizer=on') */ ...`.

---

## 5. `EXPLAIN ANALYZE FORMAT=JSON INTO @var` — lưu plan để đối chiếu

✅ verified.

```sql
EXPLAIN ANALYZE FORMAT=JSON INTO @plan SELECT COUNT(*) FROM payment WHERE company_id = 1;
SELECT @plan;
```

Dùng khi cần bằng chứng before/after cho story tối ưu query — lưu plan trước và sau vào file, so số `actual time`/`rows`. Không nhúng vào code sản phẩm.

---

## 6. Authentication — `mysql_native_password` đã bị XÓA

✅ verified: `information_schema.PLUGINS` chỉ còn `caching_sha2_password` + `sha256_password`.

- DSN/config nào còn ép `mysql_native_password` → connect fail, không phải lỗi app.
- 9.7 thêm định dạng lưu **PBKDF2** cho `caching_sha2_password` — không cần sửa client.
- LDAP SASL: `SCRAM-SHA-1` deprecated, mặc định nay là `SCRAM-SHA-256`.
- ✅ đã scan repo: không chỗ nào tham chiếu `mysql_native_password`.

---

## 7. Thay đổi hành vi 9.7.0 cần soi khi upgrade

| Thay đổi | Ảnh hưởng Cluvix |
|---|---|
| **`MD5()` / `SHA1()` không còn là built-in** → bị coi là stored function (`ERROR 1305: FUNCTION <db>.MD5 does not exist`) | Dùng `SHA2(x, 256)`. Đã grep: 0 chỗ trong SQL của repo (Go dùng `crypto/md5` phía app) |
| `IN (...)` chứa `NULL` — sửa kết quả sai | Query `IN ?` với slice rỗng GORM sinh `IN (NULL)` → xác nhận lại kết quả filter (rule cũ trong `skills/writing-mysql-migrations/SKILL.md` vẫn đúng: không match row nào) |
| `TIMEDIFF(datetime, date)`, `FROM_DAYS()`, `DAYNAME()+0`, `ADDDATE()` năm 0 | Dự án lưu timestamp `BIGINT` Unix → ít rủi ro; chỉ soi chỗ raw SQL có hàm date |
| Gán `DATE` → cột INT nay cast thẳng, không mở rộng sang DATETIME | Soi backfill SQL nào ghi date vào cột bigint |
| UNIQUE NOT NULL được coi là PK-equivalent (GIPK / `SQL_REQUIRE_PRIMARY_KEY`) | Không ảnh hưởng — mọi bảng đều có `id` PK |
| `LOCK=NONE` nay drop được cột trên bảng có virtual column | Migration DROP COLUMN trên bảng có generated column bớt lock |
| Bỏ biến `replica_parallel_type`, `group_replication_allow_local_lower_version_join` | Chỉ ảnh hưởng nếu deploy có replication |
| Default `innodb_log_writer_threads` nay phụ thuộc `log_bin` + số CPU | Chỉ tuning; không đổi nếu đã set tay |
| `binlog_transaction_dependency_history_size` default 25.000 → 1.000.000 | Tăng RAM nhẹ trên server bật binlog |

---

## 8. Có nhưng CHƯA dùng trong dự án

| Feature | Trạng thái trên 9.7.1 local | Kết luận |
|---|---|---|
| `VECTOR(n)` data type | ✅ tạo được | Chưa có nhu cầu; nếu làm semantic search bệnh án thì đây là chỗ dùng. Không index được, không làm key |
| Stored program `LANGUAGE JAVASCRIPT` | ❌ `ERROR 6001 Language component: Not available` (build Homebrew không kèm component) | Không dùng — logic ở Go, không đẩy xuống DB |
| JSON Duality Views (DML nay có ở Community) | Cú pháp `CREATE JSON DUALITY VIEW` chưa dựng được trên build này | Không dùng — API layer đã do Go đảm nhiệm, dùng duality view sẽ bypass tenant scope |
| Dynamic Data Masking (`CREATE MASKING POLICY`, `CURRENT_ROLE_IN()`) | ❌ Enterprise Edition | Che dữ liệu bệnh nhân vẫn phải làm ở tầng Go/permission |
| Masking functions (`mask_inner`...) | ❌ chưa cài component | — |
| Telemetry component (nay Community) | ❌ chưa cài (`mysql.component` rỗng) | Cân nhắc khi cần observability DB |

---

## 9. Lẫn collation — ĐÃ VÁ trên dev (2026-08-13), CHƯA vá trên DB khách

> **Trạng thái:** dev `cluvix_erp` đã convert xong (26 bảng), `collation-scan --live` exit 0.
> **DB khách production CHƯA đụng** — xem mục 10.
> Giữ nguyên phần mô tả bên dưới vì nó là **hiện trạng của mọi DB chưa vá** và là cách tái hiện lỗi.

Đo trên DB dev `cluvix_erp` TRƯỚC khi vá:

| Collation | Số bảng | Xung đột với `utf8mb4_unicode_ci`? |
|---|---|---|
| `utf8mb4_unicode_ci` (chuẩn dự án) | 319 | — |
| `utf8mb4_0900_ai_ci` (rơi vào default do quên khai) | 13 | ❌ **Có** |
| `utf8mb4_general_ci` (legacy) | 12 | ❌ **Có** |
| `utf8mb3_unicode_ci` (legacy `rbac_auth_*`) | 3 | ✅ Không (coercible lên utf8mb4) |

✅ verified trên `cluvix_erp`:
```sql
SELECT ... FROM bank_accounts b JOIN customer c ON b.bank_code = c.code;
-- ERROR 1267: Illegal mix of collations (utf8mb4_0900_ai_ci,IMPLICIT) and (utf8mb4_unicode_ci,IMPLICIT)
SELECT bank_code FROM bank_accounts UNION ALL SELECT code FROM customer;
-- ERROR 1271: Illegal mix of collations for operation 'UNION'
SELECT ... FROM partners p JOIN customer c ON p.name = c.code;
-- ERROR 1267 — general_ci cũng xung đột, không riêng 0900
SELECT ... FROM rbac_auth_item i JOIN customer c ON i.name = c.code;  -- OK, utf8mb3 không xung đột
```

**Đã cắn dự án 1 lần:** `webapp/emr/patient_document/handler/summary.go:128-174` phải ép `COLLATE utf8mb4_unicode_ci` thủ công lên **mọi** cột string của 4 nhánh UNION vì `emr_treatment_followup`/`emr_nursing_note` rơi vào collation mặc định.

**Sửa file migration KHÔNG sửa bảng đã apply.** `20260415141827_create_bank_accounts.up.sql` ghi đúng `COLLATE=utf8mb4_unicode_ci` nhưng bảng thật vẫn `utf8mb4_0900_ai_ci` (file được sửa sau khi đã apply) → phải có migration `ALTER ... CONVERT TO` riêng.

- **Client `mysql` 9.x**: mặc định `collation_connection = utf8mb4_0900_ai_ci` → so **cột với literal** cũng lỗi 1267, không cần JOIN.
- Mọi file SQL chạy tay trên DB này mở đầu bằng `SET NAMES utf8mb4 COLLATE utf8mb4_unicode_ci;`
- **RULE (vẫn áp dụng mãi):** mọi `CREATE TABLE` mới BẮT BUỘC khai `COLLATE=utf8mb4_unicode_ci` tường minh. 11 file migration đã từng vi phạm — nay có lưới tự động: `python3 backend/scripts/collation-scan.py --static` (exit ≠ 0 nếu tái phạm).
- **`utf8mb3`** (3 bảng `rbac_auth_*`) đã convert sang utf8mb4 ở dev; công thức DROP FK → CONVERT (bảng cha trước) → ADD FK nằm trong `migrations/20260813110932_rbac_utf8mb3_to_utf8mb4.up.sql`.
- **Hai đường thừa kế** (nguyên nhân gốc, verify thực nghiệm): `CREATE TABLE` có `CHARSET` nhưng thiếu `COLLATE` → default của **charset** (`utf8mb4_0900_ai_ci`); không có mệnh đề charset nào → default của **DATABASE**. Vá cả hai mới hết rò rỉ — migration `20260813105700` có `ALTER DATABASE` cho vế thứ hai.
- Hồ sơ đầy đủ (plan, 5 story, 2 pass QA, nợ): `bmad/done/mysql97-backend-alignment/`.

---

## 10. Chưa làm — BẮT BUỘC trước khi apply lên DB khách

Prod cũng chạy 9.7 (chốt 2026-08-13), nên không còn khác biệt dev↔prod về version. Nhưng migration
collation **chưa apply lên khách nào**; 3 việc dưới đây phải làm trước, mỗi DB khách một lần:

1. **Đo drift**: `python3 backend/scripts/collation-scan.py --live <db>` (chỉ đọc). Con số của dev
   KHÔNG suy ra được cho khách — DB-per-customer, default collation mỗi DB cũng có thể khác.
2. **Kiểm quyền**: `SHOW GRANTS FOR CURRENT_USER;` với user migrate. Migration chạy `ALTER DATABASE`,
   cần quyền `ALTER` mức schema. Thiếu → migration fail → `schema_migrations` dirty → **chặn deploy**.
3. **Đo thời gian `CONVERT TO`** trên bảng lớn (`attachment`, `emr_audit_log`): nó rebuild toàn bảng,
   khoá ghi. Dev gần như rỗng nên số đo ở dev vô nghĩa.

**Số đo thật trên 13 DB khách prod** (2026-08-16, quét dump 13 DB khách — tên khách thật đã ẩn trước khi public, F4):
**4609 cột `utf8mb4_unicode_ci` / chỉ 12 cột lệch** (8 `general_ci` + 4 `0900_ai_ci`), **0 `utf8mb3`**, 0 routine/event.
→ Khách drift ít hơn dev (26 bảng) rất nhiều; migration collation lên khách gần như no-op, **việc 3 hạ ưu tiên**.
Vẫn phải chạy việc 1 từng DB để biết 12 cột đó nằm ở bảng nào.

**Hệ quả người dùng sẽ thấy sau khi apply:** thứ tự sắp xếp `Đ`/`D` đổi ở các bảng vừa convert
(`general_ci`/`0900_ai_ci` xếp `Đông` trước `Dung`, `unicode_ci` thì ngược lại). Không phải bug.

Nợ liên quan chưa xử: gỡ workaround ép `COLLATE` thủ công trong
`webapp/emr/patient_document/handler/summary.go` — CHỈ gỡ khi mọi DB khách đã convert xong.

---

## Nguồn

- [MySQL 9.7 Reference Manual — What Is New](https://dev.mysql.com/doc/refman/9.7/en/mysql-nutshell.html)
- [Changes in MySQL 9.7.0](https://dev.mysql.com/doc/relnotes/mysql/9.7/en/news-9-7-0.html)
- [FOREIGN KEY Constraint Differences (9.7)](https://dev.mysql.com/doc/refman/9.7/en/ansi-diff-foreign-keys.html)
- [Hypergraph optimizer in 9.7 Community](https://blogs.oracle.com/mysql/the-hypergraph-optimizer-is-now-available-in-mysql-9-7-community-edition)
