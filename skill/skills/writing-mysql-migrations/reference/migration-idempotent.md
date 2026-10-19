# Migration idempotent — patterns chi tiết

## Mục lục

- Idempotent migrations (CRITICAL)
  - Pattern đúng: `SET @sql = IF(...) + PREPARE/EXECUTE`
  - INFORMATION_SCHEMA lookup
  - NOT NULL + FK — KHÔNG dùng DEFAULT 0 (CRITICAL)
  - `CREATE PROCEDURE` — DÙNG ĐƯỢC (cần `IF/THEN`, vòng lặp, `SIGNAL`)
- golang-migrate gotchas
- Auto-recovery dirty state

---


Tách khỏi `skills/writing-mysql-migrations/SKILL.md` (2026-08-13, file vượt 350 dòng). Quy ước chung, decimal, FK, template
`CREATE TABLE` → xem `skills/writing-mysql-migrations/SKILL.md`. Khả năng/giới hạn riêng của **MySQL 9.7** → `skill/skills/writing-mysql-migrations/reference/mysql97.md`.

## Idempotent migrations (CRITICAL)

**Mọi ALTER TABLE PHẢI idempotent** — re-run trên DB đã apply không được fail.

**Seed data cũng phải idempotent**: `INSERT IGNORE INTO` hoặc `INSERT ... ON DUPLICATE KEY UPDATE`, KHÔNG raw `INSERT INTO ... VALUES(id cố định)`. Raw INSERT với id cố định chạy lại lên DB đã có row → dup-key → migrate dirty → chặn toàn bộ deploy (đúng sự cố `seed_data.up.sql` 2026-07-18, xem mục ⛔ trong `skills/writing-mysql-migrations/SKILL.md`).

**`INSERT IGNORE` CHỈ idempotent khi bảng có UNIQUE key trên cột định danh** — bảng thiếu unique (DB thật có thể drift khỏi baseline, vd `setting.key` không có unique dù baseline khai) → `INSERT IGNORE` chạy lại tạo ROW TRÙNG câm lặng. Seed theo key business không có unique constraint → dùng `INSERT ... SELECT ... WHERE NOT EXISTS (...)` (tiền lệ: seed `cron_job` theo `handler_key` — bảng đã drop 2026-10-17). Verify unique tồn tại thật (`SHOW INDEX`) trước khi chọn `INSERT IGNORE` (bài học `20260721205526`); verify idempotent: chạy raw SQL 2 lần, count không đổi.

**"Chỉ 1 row active per key" — MySQL KHÔNG có partial/filtered unique index**: dùng **cột GENERATED STORED = giá-trị-khi-active / NULL-khi-không** + UNIQUE gồm cột đó (NULL≠NULL nên nhiều row inactive không đụng nhau, terminal tự nhả UNIQUE cho lần active mới). VD `active_key TINYINT GENERATED ALWAYS AS (IF(status='running',1,NULL)) STORED` + `UNIQUE(doc_type,doc_id,active_key)` (approval_instance: 1 phiếu chỉ 1 instance running); hoặc `NULLIF(col,'')` khi "không active" = giá trị rỗng (zns_recipient `phone_key`/`uid_key` cho recipient UID-only phone=''). Cột generated là DB-managed — KHÔNG map vào struct Go.

MySQL KHÔNG support trong ALTER TABLE (đã verify lại trên **9.7.1** — vẫn `ERROR 1064`, đừng tin tin đồn "MySQL 9 đã có"):
- ❌ `ADD COLUMN IF NOT EXISTS`
- ❌ `DROP COLUMN IF EXISTS`
- ❌ `DROP FOREIGN KEY IF EXISTS`
- ❌ `ADD CONSTRAINT IF NOT EXISTS`
- ❌ `ADD KEY IF NOT EXISTS` / `DROP INDEX IF EXISTS` trong ALTER TABLE

**Tên index thật** lấy từ baseline migration / `SHOW INDEX FROM <table>` — KHÔNG suy từ gorm tag. Guard information_schema với tên sai = silent no-op (migrate vẫn xanh, index cũ còn nguyên) → sau migrate phải `SHOW INDEX` verify.

**DROP INDEX đang backing 1 FK → Error 1553**: MySQL không cho drop index nếu FK constraint đang dựa vào nó mà không có index khác cover được cột đó. Khi đổi UNIQUE đơn cột → composite (vd `branch_id` → `(branch_id, company_id)`) và cột đơn có FK riêng: PHẢI `ADD` index/unique mới TRƯỚC, `DROP` index cũ SAU (thứ tự ngược sẽ fail).

### Pattern đúng: `SET @sql = IF(...) + PREPARE/EXECUTE`

Dùng cho việc đơn (add/drop column, FK, index) — ngắn hơn procedure.

```sql
-- Add column nếu chưa tồn tại
SET @sql = IF(
  (SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
    WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='table_name' AND COLUMN_NAME='new_column') = 0,
  'ALTER TABLE table_name ADD COLUMN new_column DECIMAL(20,2) DEFAULT 0.00 COMMENT ''Mô tả'' AFTER existing_column',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- Drop column nếu tồn tại
SET @sql = IF(
  (SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
    WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='table_name' AND COLUMN_NAME='new_column') > 0,
  'ALTER TABLE table_name DROP COLUMN new_column',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- Drop FK nếu tồn tại (PHẢI drop FK TRƯỚC drop column liên quan)
SET @sql = IF(
  (SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS
    WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='table_name' AND CONSTRAINT_NAME='fk-table_name-col_id') > 0,
  'ALTER TABLE table_name DROP FOREIGN KEY `fk-table_name-col_id`',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- Add FK nếu chưa tồn tại
SET @sql = IF(
  (SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS
    WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='table_name' AND CONSTRAINT_NAME='fk-table_name-col_id') = 0,
  'ALTER TABLE table_name ADD CONSTRAINT `fk-table_name-col_id` FOREIGN KEY (col_id) REFERENCES parent(id) ON DELETE RESTRICT ON UPDATE CASCADE',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
```

Có thể gộp `ADD COLUMN` + `ADD INDEX` + `ADD CONSTRAINT` trong 1 ALTER TABLE.
Quote escape trong string literal: `''` (2 dấu nháy).

### INFORMATION_SCHEMA lookup

| Đối tượng | Bảng query | Cột match |
|-----------|-----------|-----------|
| Column | `INFORMATION_SCHEMA.COLUMNS` | `TABLE_SCHEMA=DATABASE() AND TABLE_NAME AND COLUMN_NAME` |
| FK | `INFORMATION_SCHEMA.TABLE_CONSTRAINTS` | `TABLE_SCHEMA=DATABASE() AND TABLE_NAME AND CONSTRAINT_NAME AND CONSTRAINT_TYPE='FOREIGN KEY'` |
| Index | `INFORMATION_SCHEMA.STATISTICS` | `TABLE_SCHEMA=DATABASE() AND TABLE_NAME AND INDEX_NAME` |
| Table | `INFORMATION_SCHEMA.TABLES` | `TABLE_SCHEMA=DATABASE() AND TABLE_NAME` |

- ⚠ MySQL 9.7: `KEY_COLUMN_USAGE` và `REFERENTIAL_CONSTRAINTS` lệch collation (`utf8mb3_tolower_ci` ≠ `utf8mb3_general_ci`) — JOIN 2 view, hoặc so biến lấy từ view này với cột của view kia ⇒ `ERROR 1267`.
- Tra FK kèm `DELETE_RULE`: chỉ query `REFERENTIAL_CONSTRAINTS` với `CONSTRAINT_SCHEMA = DATABASE() AND TABLE_NAME = '<literal>' AND REFERENCED_TABLE_NAME = '<literal>'`, lấy bằng `SELECT CONSTRAINT_NAME, DELETE_RULE INTO @fkname, @fkrule … LIMIT 1`.

### NOT NULL + FK — KHÔNG dùng DEFAULT 0 (CRITICAL)

```sql
-- 1. Add NULL trước
SET @sql = IF(
  (SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
    WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='table_name' AND COLUMN_NAME='company_id') = 0,
  'ALTER TABLE table_name ADD COLUMN company_id INT NULL COMMENT ''Thương hiệu'' AFTER existing_col',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 2. Backfill
UPDATE table_name t
  JOIN parent_table p ON p.id = t.parent_id
SET t.company_id = p.company_id
WHERE t.company_id IS NULL;

-- 3. Enforce NOT NULL
ALTER TABLE table_name MODIFY COLUMN company_id INT NOT NULL COMMENT 'Thương hiệu';

-- 4. Add FK idempotent (xem pattern trên)
```

### `CREATE PROCEDURE` — DÙNG ĐƯỢC (cần `IF/THEN`, vòng lặp, `SIGNAL`)

DELIMITER là khái niệm **client**; `cmd/migrate/main.go:120` bật `multiStatements=true` nên server nhận nguyên khối và parse thân procedure đúng. Tiền lệ: `20260427150000_rbac_schema_fix.up.sql`, `20260813110932_rbac_utf8mb3_to_utf8mb4.up.sql`.

```sql
DROP PROCEDURE IF EXISTS _migrate_x;
CREATE PROCEDURE _migrate_x()
BEGIN
  IF <điều kiện information_schema> THEN
    ALTER TABLE ...;
  END IF;
  IF <phát hiện dữ liệu xấu> THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'mo ta loi ro rang';
  END IF;
END;
CALL _migrate_x;
DROP PROCEDURE IF EXISTS _migrate_x;
```

- `SIGNAL SQLSTATE` KHÔNG chạy trong `PREPARE` (`ERROR 1295`) — cần fail có thông điệp thì phải dùng procedure.
- Precheck dữ liệu (trùng khoá sau đổi collation…) đặt TRƯỚC mọi `DROP FK`/`ALTER`, không thì fail giữa chừng để lại schema dở dang.

## `CAST(... AS UNSIGNED)` trong điều kiện JOIN làm abort CẢ migration

Server bật `STRICT_TRANS_TABLES` ⇒ `CAST('abc' AS UNSIGNED)` là **`ERROR 1292 Truncated incorrect
INTEGER value`**, KHÔNG phải cảnh báo. Một dòng rác trong cột JSON đủ làm migration dừng giữa chừng →
golang-migrate ghi dirty state → chặn toàn bộ deploy. Hỏng im lặng cho tới lúc deploy.

Thêm guard `JSON_TYPE(...) = 'INTEGER'` vào `WHERE` **KHÔNG cứu được** (đã thử, vẫn `ERROR 1292`):
optimizer đánh giá điều kiện JOIN **trước** mệnh đề WHERE, không có cách bảo đảm thứ tự.

Cách đúng — so sánh JSON trực tiếp, bỏ hẳn `CAST`:

```sql
-- SAI: rác trong companyId là abort cả migration
JOIN company c ON c.id = CAST(JSON_UNQUOTE(JSON_EXTRACT(u.companyId, '$[0]')) AS UNSIGNED)
-- ĐÚNG: không bao giờ lỗi
JOIN company c ON c.id = JSON_EXTRACT(u.companyId, '$[0]')
```

Bonus: so sánh JSON khớp đúng tập giá trị mà Go `json.Unmarshal` vào `[]uint` đọc được
(`[3]` khớp; `["3"]`/`["abc"]`/`[null]`/`[true]`/`[1.9]` không) ⇒ migration và runtime nhìn dữ liệu
giống hệt nhau, không sinh ra lớp bản ghi "migration gán được nhưng runtime không đọc được".

## golang-migrate gotchas
- File có `CREATE PROCEDURE` chỉ chạy qua golang-migrate — `mysql < file.sql` cho `ERROR 1064` (client cắt input theo `;`, vỡ thân procedure)
- Test idempotent file procedure: `UPDATE schema_migrations SET version=<trước>, dirty=0` rồi `migrate up` lại — KHÔNG dùng `mysql < file`
- Mỗi statement kết thúc bằng `;` — **server** parse multi-statement, tool KHÔNG split theo `;`
- Gộp nhiều migration thành 1 file: nối bằng `cat` theo thứ tự version là tương đương ngữ nghĩa (golang-migrate và `testhelpers.execFile` đều gửi nguyên file 1 `COM_QUERY`, chung 1 connection, không bọc transaction). Điều kiện: mọi file nguồn phải kết thúc bằng `;` và có newline cuối — thiếu là câu lệnh dính sang file sau
- Quote escape: `''` thay vì `\'`
- Không inline comment giữa SQL statement


## Auto-recovery dirty state

`cmd/migrate/main.go` tự detect dirty=1 → force previous → retry.
**Yêu cầu migration idempotent** — re-run không fail.

⚠ **`dirty = 1` KHÔNG có nghĩa migration chưa chạy xong.** golang-migrate ghi cờ **TRƯỚC** khi chạy
và chỉ xoá **SAU** khi xong, nên process chết ở giữa để lại cờ kể cả khi toàn bộ SQL đã apply
(gặp 2026-09-08: `20260907195602` kẹt dirty nhưng đủ cả 2 cột, `NOT NULL`, index, 4 FK).

Trước khi chọn đích `force`, kiểm **từng bước** của migration đang kẹt xem đã có trong DB chưa
(cột · `NOT NULL` · index · FK · backfill):
- đủ hết ⇒ force tới **CHÍNH version đó** (chỉ ghi lại sự thật);
- thiếu bước nào ⇒ force về version **TRƯỚC** để chạy lại.

Force về previous theo phản xạ chỉ an toàn khi file idempotent — file squash thì không
(xem `../SKILL.md`).

Hook `guard-bash.sh` chặn agent chạy `-cmd=force` trên `DB_NAME`; `-cmd=up` thì cho qua ⇒ lệnh
force phải do người tự gõ.
