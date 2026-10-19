---
name: writing-mysql-migrations
description: >-
  Viết migration MySQL cho Cluvix (golang-migrate, Go + GORM): quy ước timestamp, idempotent
  ADD/DROP COLUMN, FK constraint thật, decimal tiền, MySQL 9.7 LTS syntax mới/cũ, và luật
  CẤM tuyệt đối test/migrate chạm DB dev/prod thật. Dùng skill này TRƯỚC KHI tạo file
  migration mới, sửa schema, thêm cột/FK, backfill dữ liệu, hoặc trước khi chạy
  `cmd/migrate`/integration test đụng tới database — kể cả migration chỉ 1-2 dòng.
---

# Migration (Go + GORM)

> Quy ước timestamp/decimal/FK/template + luật CẤM chạm DB thật nằm ngay trong file này.
> `reference/migration-idempotent.md` — chi tiết pattern idempotent ADD/DROP COLUMN/FK/INDEX.
> `reference/mysql97.md` — cú pháp MySQL 9.7 LTS mới dùng được / vẫn cấm / cố ý không dùng.
> `reference/upgrade-lts.md` — ghi chép quá trình upgrade lên MySQL 9.7 LTS.


> **Server đang chạy MySQL 9.7 LTS (dev + prod).** Trước khi viết migration, đọc `skill/skills/writing-mysql-migrations/reference/mysql97.md` nếu
> định dùng cú pháp/tính năng mà file này chưa nói tới — nó ghi rõ cái gì 9.7 **mới** dùng được
> (functional unique index, `SIGNAL` trong procedure, hypergraph), cái gì **vẫn không** dùng được
> (`ADD COLUMN IF NOT EXISTS` vẫn `ERROR 1064`), và cái gì 9.7 cho phép nhưng dự án **cố ý cấm**
> (inline `REFERENCES` nay tạo FK thật nhưng tên `_ibfk_N` phá guard idempotent).
> Quy ước hằng ngày (timestamp, decimal, FK, template) thì ở ngay file này — không cần đọc `mysql97.md`.

## Tool
- `golang-migrate`: `migrate -path migrations/ -database "mysql://..." up`
- Tool luôn áp DB theo `DB_NAME` trong `.env` — verify bằng `mysql` cli thủ công phải connect ĐÚNG DB đó, không phải DB nào khác đang mở sẵn (dễ nhầm "chưa lên", "không hit")
- Files: `backend/migrations/{timestamp}_{description}.up.sql` — **CHỈ tạo file `.up.sql`, KHÔNG tạo `.down.sql`**
- Tạo file **bằng tay** — KHÔNG dùng `-cmd=create`: nó vẫn sinh kèm `.down.sql` (trái rule, phải xoá tay)
- `-cmd=down` là code chết: repo không có `.down.sql` nào nên `m.Steps(-n)` luôn lỗi "file does not exist". Không có đường rollback bằng tool — sai thì viết migration mới bù
- Timestamp: `YYYYMMDDHHmmss` — 14 ký tự đến giây (VD: `20260520150544`)
- **Trước khi tạo**: `ls migrations/ | sort | tail -3` — timestamp mới phải lớn hơn file mới nhất
- Dùng `date +%Y%m%d%H%M%S` nếu kết quả lớn hơn file mới nhất; nếu không thì tăng từ file mới nhất
- Không dùng số tròn như `130000`, `000000` — dễ trùng
- **Trước khi `migrate up`**: chạy lại `ls migrations | sort | tail -3` — nếu có file mới hơn timestamp của mình (agent/session khác vừa tạo+apply song song), phải regenerate timestamp và rename file trước khi apply, nếu không `migrate up` sẽ báo "không có migration mới" và bỏ qua âm thầm (version DB đã vượt qua timestamp cũ của mình).
- **Cổng thật là DB, không phải danh sách file**: `SELECT MAX(version) FROM schema_migrations` trên **DB đích** (mỗi DB khách một con số khác nhau). `ls` chỉ thấy file trong checkout của mình — DB có thể đã vượt qua timestamp đó do phiên/máy khác apply. Tên file ≤ version hiện tại ⇒ golang-migrate **bỏ qua vĩnh viễn**: không lỗi, không cảnh báo, và nếu đó là migration dọn dữ liệu sai thì dữ liệu sai nằm lại mãi.
- **KHÔNG hỏi được "migration X đã apply chưa" từ `schema_migrations`** — bảng chỉ có MỘT dòng (`version` + `dirty`), không phải nhật ký; `WHERE version IN (…)` trả 0 KHÔNG có nghĩa chưa apply. Kiểm bằng **hiện vật**: `information_schema.STATISTICS` / `COLUMNS` xem chỉ mục/cột đã tồn tại và đủ cột chưa.
- **Trước mỗi `-cmd=up`**: liệt kê file có version > `MAX(version)` của DB đích để biết chính xác sắp áp gì — `-cmd=up` áp TẤT CẢ file pending, gồm cả việc dang dở của phiên khác.
- `migrations/` (schema mới CRM standalone, story-62): mỗi bảng 1 file `20260928093138…_create_<bảng>.up.sql` (`CREATE TABLE IF NOT EXISTS`, idempotent) + `…_add_fk_appointment` + 4 file seed `2026092809471x_seed_{business_profile,location,lookup,admin}` + migration viết sau đó. KHÔNG còn baseline/squash.
- (Lịch sử, bộ ERP cũ — nay nằm ở `backend/migrations_backup/`, migrate KHÔNG đọc: `00000000000000_baseline` + `seed_data` + `fix_json_string_columns` + `20260814091602_squash_20260406_to_20260814` gộp 326 migration. Hai gạch đầu dưới chỉ áp cho DB còn chạy bộ cũ.)
- **TUYỆT ĐỐI KHÔNG đổi version của file squash.** `schema_migrations` chỉ giữ MỘT con số: giữ `20260814091602` → DB ở head `ErrNoChange`; đổi sang timestamp mới → mọi DB ở head chạy lại toàn dải và chết ở đoạn `20260427120000` (`Error 1054 company_type_id`), để lại dirty
- File squash KHÔNG idempotent (72/326 migration gốc 04–07/2026 dùng `CREATE TABLE` trần / `ADD COLUMN` không guard) ⇒ `-auto-recover` vô dụng với nó. Provisioning DB mới hỏng giữa chừng → `DROP DATABASE` làm lại (~25s), KHÔNG force

## ⛔ CẤM test/migrate chạm DB thật — CATASTROPHIC (bài học 2026-07-18)

**Sự cố:** trong 1 session BMAD dài fan-out nhiều subagent tự verify integration test, DB dev thật của 1 khách (`DB_NAME` trong .env) bị **wipe sạch về baseline** (mất toàn bộ bảng + data) rồi kẹt `schema_migrations` **dirty ở version 1** (chết khi apply `seed_data.up.sql`), chặn toàn bộ ~250 migration sau. Không dựng lại được thủ phạm chính xác vì migrate stderr không được log.

**Nguyên nhân gốc (3 lớp, không phải 1 lệnh đơn):**
1. **`testhelpers.New` (`internal/testhelpers/suite.go`) gọi `dropAllTables` = `DROP TABLE` MỌI bảng** trong DB mà `TEST_MYSQL_DSN` trỏ tới. Trỏ nhầm vào DB thật = mất sạch data ngay, không undo.
2. **`seed_data.up.sql` KHÔNG idempotent** (raw dump `INSERT INTO ... VALUES(id cố định)`) → chạy lại lên DB đã có data → dup-key → migrate **dirty** → chặn deploy.
3. **Plan/story ghi thẳng tên DB dev thật (`DB_NAME` trong .env) làm target verify backfill** → cấp "giấy phép" cho agent trỏ migrate/test vào DB thật. Cộng root creds trong memory + fan-out cô lập, mỗi subagent tự tạo DB → không ai đặt guard.

**LUẬT (áp cho MỌI agent, kể cả fix nhỏ):**
- Integration test / `testhelpers.New` / `cmd/migrate` khi verify **CHỈ** chạy trên **DB dùng-một-lần** (`CREATE DATABASE ..._test/_tmp` đầu → `DROP` cuối). **TUYỆT ĐỐI KHÔNG** trỏ `TEST_MYSQL_DSN` hay `-db` vào `DB_NAME` (DB dev/prod thật).
- **Plan/story KHÔNG BAO GIỜ ghi tên DB thật làm target verify.** Cần data thật để kiểm backfill → clone: `CREATE DATABASE cluvix_verify_tmp` + nạp dump → verify → `DROP`. Không đụng bản thật.
- **Mọi migration idempotent** — kể cả seed: `INSERT IGNORE` / `ON DUPLICATE KEY UPDATE`, không raw `INSERT` id cố định (xem mục Idempotent bên dưới).
- Guard code đã cài (lưới cuối, KHÔNG thay kỷ luật): `testhelpers.New` **fatal nếu DB trong DSN == `DB_NAME`**; `seed_data` đã `INSERT IGNORE`; `cmd/migrate` `force`/`down` cần `-confirm`. **`-auto-recover` KHÔNG cần `-confirm`** — là flag opt-in tường minh cho deploy tự phục hồi dirty-state chạy nền; bắt `-confirm` sẽ phá pipeline deploy (regression 2026-07-18).

**Nếu chuyện này xảy ra trên PRODUCTION (mô hình DB-per-customer):**
- **Mất toàn bộ dữ liệu 1 khách hàng ngay lập tức**: `dropAllTables` xóa mọi bảng của DB đó — customer, hóa đơn, bệnh án EMR, sổ kế toán, bảng lương — **không có undo tầng app**. Blast radius = trọn 1 khách (không leak cross-tenant nhưng khách đó mất sạch). Nếu script "migrate all tenants" loop qua mọi DB → **mất TẤT CẢ khách**.
- **Migration dirty = outage**: app không boot / deploy kế bị chặn cho tới khi có người xử tay `force`.
- **Không backup = mất vĩnh viễn**; có backup thì vẫn mất phần data từ mốc backup gần nhất (RPO gap).
- **Bắt buộc trước khi bất cứ thứ gì chạm gần prod**: (a) creds/network test tách hẳn prod — code test *không thể* mở nổi connection tới prod; (b) backup tự động + **đã test restore** (binlog/PITR); (c) migration idempotent + cổng dry-run staging; (d) `dropAllTables` chỉ chạy được trên DB tên `*_test`. Guard `DB_NAME`-equality ở trên là điều kiện cần tối thiểu, chưa đủ cho prod.

## Decimal columns
- Số tiền lớn (payment, revenue, debt): `decimal(20,2)`
- Số tiền nhỏ/giá thuốc/giá vật tư: `decimal(15,2)`
- Lương: `decimal(15,0)`
- Phần trăm/tỷ lệ: `decimal(5,2)`
- Chỉ số sinh tồn (BMI, cân nặng): `decimal(5,2)`
- Ngày công/giờ: `decimal(5,1)` hoặc `decimal(6,1)`
- KHÔNG dùng `float` hay `double` cho tiền

## GORM tag phải khớp migration
```go
// Tiền/decimal: decimal.Decimal (shopspring), KHÔNG float64
Amount decimal.Decimal `gorm:"type:decimal(20,2);not null;default:0.00" json:"amount"`
ImportPrice *decimal.Decimal `gorm:"column:import_price;type:decimal(15,2);default:0.00" json:"import_price"`
Percentage *decimal.Decimal `gorm:"type:decimal(5,2)" json:"percentage"`
```

## Column conventions
- FK nullable: `INT NULL` + `INDEX` + `FOREIGN KEY ... ON DELETE SET NULL` (hoặc `RESTRICT` nếu business không cho phép mất tham chiếu)
- FK not null: `INT NOT NULL` + `INDEX` + `FOREIGN KEY ... ON DELETE RESTRICT` (mặc định — không cho xóa parent khi còn con; chỉ dùng `CASCADE` khi đã xác nhận business muốn xóa dây chuyền)
- Timestamp: `BIGINT` (Unix), KHÔNG `DATETIME`
- JSON: `JSON` type, KHÔNG `TEXT`
- String có giới hạn: `VARCHAR(n)`, KHÔNG `TEXT` trừ khi cần
- `created_at`, `updated_at`: `BIGINT` (Unix)
- `status`: `TINYINT(1) DEFAULT 1`
- KHÔNG thêm cột `version` — optimistic lock theo `updated_at` (D6/D21, tập bảng ở `pkg/optlock`)
- `sort_order`: `INT DEFAULT 0`
- `branch_id`: `INT` + `INDEX` + FK tới `branch` (NULL = dùng chung mọi chi nhánh — Hard Rules). KHÔNG còn cột `company_id` (D15/story-62: 1 thương hiệu/DB).

## FK phải khớp signed/unsigned với id bảng cha (Error 3780)

- Kiểu cột FK PHẢI khớp CHÍNH XÁC signed/unsigned với id bảng cha — lệch → MySQL Error 3780 khi ADD FK, chặn toàn chuỗi migrate.
- Schema hiện tại: `id` các bảng cha (`appointment`, `customer`, `branch`, `user`…) là `int` SIGNED; bảng mới hay quen tay `int unsigned` → kiểm trước: `grep -m1 '\`id\`' migrations/*_create_<bảng cha>.up.sql`.
- Kiểm ĐỦ mọi cột FK trong file (kể cả `measured_by`/`created_by`) — sót 1 cột vẫn chặn migrate.

## CREATE VIEW phải khai DEFINER tường minh

`CREATE [OR REPLACE] VIEW` thiếu `DEFINER` → MySQL tự gán definer = user chạy migration lúc tạo lần đầu. Máy/DB khác không có đúng user đó → mọi `SELECT` qua view lỗi `Error 1449 (HY000): The user specified as a definer ... does not exist`. Luôn khai `CREATE OR REPLACE DEFINER=CURRENT_USER VIEW ...` (không hardcode username, chạy đúng trên mọi máy).

## Template up.sql
```sql
CREATE TABLE `table_name` (
  `id` int unsigned NOT NULL AUTO_INCREMENT,
  `branch_id` int DEFAULT NULL COMMENT 'Chi nhánh (NULL = dùng chung)',
  `category_id` int unsigned DEFAULT NULL COMMENT 'FK ví dụ — tham chiếu bảng khác',
  `status` tinyint(1) DEFAULT '1',
  `amount` decimal(20,2) NOT NULL DEFAULT '0.00' COMMENT 'Số tiền',
  `note` text COMMENT 'Ghi chú',
  `created_at` bigint DEFAULT NULL,
  `updated_at` bigint DEFAULT NULL,
  `created_by` int DEFAULT NULL,
  `updated_by` int DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_branch_id` (`branch_id`),
  KEY `idx_category_id` (`category_id`),
  CONSTRAINT `fk-table_name-branch_id` FOREIGN KEY (`branch_id`) REFERENCES `branch` (`id`) ON DELETE RESTRICT ON UPDATE CASCADE,
  CONSTRAINT `fk-table_name-category_id` FOREIGN KEY (`category_id`) REFERENCES `category` (`id`) ON DELETE RESTRICT ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

### FOREIGN KEY bắt buộc cho mọi cột tham chiếu bảng khác (CRITICAL)

Mọi cột dạng `xxx_id` tham chiếu tới 1 row cụ thể ở bảng khác — kể cả self-reference (`parent_id`) — **PHẢI có `FOREIGN KEY` thật trong CREATE TABLE** (không chỉ `KEY`/`INDEX`). Áp dụng CẢ `branch_id` (schema mới story-62: các bảng có `branch_id` như `customer`, `appointment`… đều có FK tới `branch`; `branch_id` NULL = dùng chung — FK cho phép NULL). Ngoại lệ DUY NHẤT: `created_by`/`updated_by` (xem CLAUDE.md). (Ngoại lệ `company_id` cũ đã bỏ cùng cột, D15.)

**Lý do (CRITICAL — không phải style, là bug thật):** `models.DeleteWithCheck` (`internal/models/base.go`) — cơ chế DUY NHẤT chặn xóa record đang bị bảng khác tham chiếu — chỉ đơn giản DELETE rồi bắt lỗi MySQL 1451 (FK violation). Bảng thiếu FK thật → hàm này không có gì để bắt lỗi → xóa "thành công" nhưng để lại con mồ côi (con vẫn giữ id đã bị xóa, không báo lỗi ở đâu cả). Case thật: `payment.cash_flow_category_id` tham chiếu `cash_flow_category.id` không có FK — phát hiện 2026-07-12.

Chọn `ON DELETE`:
- `RESTRICT` (mặc định) — không cho xóa parent khi còn con, đúng nghĩa `DeleteWithCheck` đang cố làm.
- `SET NULL` — chỉ khi cột nullable và business chấp nhận con "mất tham chiếu" khi cha bị xóa (vd optional category).
- `CASCADE` — chỉ khi đã xác nhận rõ business muốn xóa dây chuyền (hiếm, rủi ro cao — cân nhắc kỹ trước khi dùng).

**Bảng đã có data:** trước khi `ADD FOREIGN KEY` (ALTER trên bảng cũ, xem pattern "Add FK nếu chưa tồn tại" ở phần Idempotent bên dưới) — BẮT BUỘC query check orphan trước:
```sql
SELECT COUNT(*) FROM child_table c
LEFT JOIN parent_table p ON c.parent_id = p.id
WHERE c.parent_id IS NOT NULL AND p.id IS NULL;
```
Có orphan → xử lý theo business (SET NULL hàng loạt / gán lại parent hợp lệ) trong CÙNG migration, TRƯỚC statement `ADD CONSTRAINT` — nếu không `ADD FOREIGN KEY` sẽ fail ngay khi migrate.

- ⚠ `ADD FOREIGN KEY` khi `FOREIGN_KEY_CHECKS=1` = thuật toán COPY ⇒ **chặn ghi cả bảng** suốt thời gian chép (153k dòng: 24s). Dữ liệu đã chắc sạch (vừa dọn orphan ngay trước trong cùng file, hoặc cột vốn có FK tới đúng bảng cha) ⇒ bọc riêng lệnh ADD: `SET @old_fkc = @@SESSION.FOREIGN_KEY_CHECKS; SET SESSION FOREIGN_KEY_CHECKS = 0;` … `SET SESSION FOREIGN_KEY_CHECKS = @old_fkc;` ⇒ INPLACE 0.02s. **Chưa dọn orphan thì cấm** — MySQL không kiểm dữ liệu cũ.
- Backfill `UPDATE … JOIN` bảng lớn: `SET SESSION TRANSACTION ISOLATION LEVEL READ COMMITTED` (không khoá đọc dòng bảng được JOIN); lưu `@@SESSION.transaction_isolation` đầu file, trả lại cuối file.

## Bỏ index thừa kiểu tiền-tố-trái — phải THỬ, không được suy luận

Index có tập cột là **tiền tố trái** của một index khác trên cùng bảng thì bỏ được, kể cả khi nó đang đỡ `FOREIGN KEY` (MySQL chấp nhận index có cột FK làm tiền tố trái). Đã bỏ 50 index như vậy ở `20260906114555` — 0 FK và 0 UNIQUE bị đụng. Loại bỏ mọi index **UNIQUE** khỏi danh sách: bỏ index unique là mất ràng buộc duy nhất, không phải tối ưu.

Ba cái bẫy, cả ba đều cho kết luận SAI mà không báo lỗi:

- `GROUP_CONCAT(COLUMN_NAME)` trên `information_schema.STATISTICS` **BỎ QUA NULL**. Index biểu thức (functional index) có `COLUMN_NAME = NULL` nên các cột biểu thức tàng hình, khiến index trông như bắt đầu bằng cột khác ⇒ kết luận "tiền tố trái" sai ⇒ `ERROR 1553 … needed in a foreign key constraint` lúc drop. Câu quét PHẢI loại index có `SUM(COLUMN_NAME IS NULL) > 0` ở **CẢ hai vế**.
- "Tiền tố trái ⇒ chắc chắn drop được" là suy luận. Chốt danh sách bằng cách **thử drop TỪNG CÁI** trên DB dùng-một-lần rồi chỉ giữ cái thành công.
- ⚠ **"Drop được" ≠ "nên drop" — phép thử trên chỉ đo FK, KHÔNG đo hiệu năng.** InnoDB nối PK vào đuôi mọi chỉ mục phụ, nên `(company_id)` thực chất là `(company_id, id)` và phục vụ `ORDER BY id DESC` miễn phí; bỏ nó đo được phân trang sâu **869ms → 4.793ms**. Chỉ mục **một cột dẫn đầu bằng cột scope** (nay là `branch_id`; số đo gốc làm trên `company_id` trước D15) trên bảng list lớn ⇒ giữ, và ghi lý do ngay trong migration để lần quét sau không bỏ lại. Số đo + cách bench: `../debugging-backend-queries/reference/perf-list-endpoints.md` §"Đừng bỏ chỉ mục một cột `(company_id)`".
- `group_concat_max_len` mặc định 1024 làm script đối chứng cắt cụt danh sách id ⇒ báo lệch giả. Nâng ở đầu phiên đo.

Chi phí đáng bỏ: một bảng từng mang index 231MB trên data 48MB (gấp 4.8 lần) vì 32 index — mỗi INSERT ghi từng ấy entry, và index không nhét vừa `innodb_buffer_pool_size`.

## Backfill phải khớp TỪNG guard của runtime, không khớp bản mô tả

Runtime luôn có điều kiện NGẦM mà bản mô tả bằng lời không nêu. Backfill viết theo lời ("tin staff sớm
nhất") **nghe đúng** mà quy kết sai — không lỗi, không cảnh báo.

- AC của story backfill phải **trích thẳng câu persist** (`file:dòng`); reviewer đối chiếu `WHERE` của
  backfill với **từng** guard runtime, không đối chiếu với câu mô tả.
- Sự cố 2026-08-20 (`omi_conversation.first_responder_id`): runtime chỉ quy kết **sau khi gửi THÀNH
  CÔNG** và **không tính tin chiến dịch** — 2 điều kiện không ai viết ra. Backfill thiếu cả 2 ⇒ **3/6**
  hội thoại trên DB dev quy kết bằng tin `status=1` (gửi lỗi). Lọt qua Dev lẫn SM, chỉ lộ khi QA đọc
  code persist thay vì đọc mô tả.
- Cùng lớp: tin của **nhiều luồng khác nhau** có thể trùng `src`/`created_by`/`status` — trước khi tin
  vào một cột phân biệt, duyệt **mọi** điểm dựng row trong code sản phẩm rồi ghi lại kết luận.

## Verify migration trên DB tạm — clone schema THẬT, không tự gõ lại bảng cha

- Dựng DB dùng-một-lần bằng `mysqldump --no-data` từ DB dev, KHÔNG tự `CREATE TABLE` lại bảng cha rút gọn.
- Bảng cha tự gõ luôn permissive hơn thật (thiếu NOT NULL/UNIQUE/kiểu cột lệch) → migration "pass" trên DB tạm rồi fail lúc chạy thật.
- Apply file migration **2 lần liên tiếp** để nghiệm thu idempotent; drop DB tạm ngay sau khi xong.
- ⚠ **Bảng có chỉ mục FULLTEXT thì clone bằng `mysqldump` CHƯA XONG ở bước nạp schema.** Dump chỉ chép
  lại *định nghĩa* index; nạp vào DB tạm là nó được dựng lại dưới `@@GLOBAL.innodb_ft_enable_stopword`
  (mặc định = 1), trong khi migration thật dựng nó với `SET SESSION … = OFF`. Hệ quả: DB tạm tìm kiếm
  **sót âm thầm** — 2026-09-07 một bench báo bản vá đúng là "sót 1.088–2.744 dòng", suýt loại nhầm.
  Phải dựng lại FULLTEXT trong DB tạm cho đúng rồi đối chứng `MATCH` = `LIKE` với DB thật TRƯỚC khi
  tin bất kỳ số nào. Quy trình dựng đúng (phải DROP HẾT fulltext trên bảng, không chỉ một cái):
  `../debugging-backend-queries/reference/patterns-gotchas.md` §"Chỉ mục ngram dựng sai stopword".
- Viết migration **tạo mới** FULLTEXT ⇒ đọc mục đó TRƯỚC: stopword gắn theo BẢNG (cố định bởi index
  FULLTEXT đầu tiên), `SET GLOBAL` KHÔNG ăn, và chỉ mục dựng sai làm tìm kiếm sai mà không ném lỗi.
- ⚠ **Đặt file vào `migrations/` = APPLY LUÔN lên DB dev**: `air` chạy `run_dev.sh` → `migrate up` vài giây sau khi file xuất hiện, không hỏi gì. Migration nặng/khoá bảng (`ADD FULLTEXT`, `ALTER` bảng lớn) phải soạn và verify ở `backend/tmp/` TRƯỚC, chỉ chuyển vào `migrations/` khi đã đạt trên DB dùng-một-lần.
  Số đo để lập cửa sổ bảo trì: `ADD FULLTEXT … WITH PARSER ngram` trên cột **TEXT tự do dài** =
  **3 phút 06 giây / 524k dòng**, gấp ~4 lần mốc ~42,7s của cột ngắn — nhân theo số khách vì
  DB-per-customer. Giảm rủi ro: code gọi `MATCH` phải tự lùi về `LIKE` khi gặp lỗi 1191, nhờ đó
  endpoint không vỡ trong lúc `ALTER` chạy và thứ tự apply không quan trọng.

## Idempotent migrations (CRITICAL)

→ `migration-idempotent.md` — `PREPARE/EXECUTE`, `CREATE PROCEDURE`, INFORMATION_SCHEMA lookup,
NOT NULL + FK, golang-migrate gotchas, auto-recovery dirty state.

## Lưu ý
- KHÔNG dùng `AutoMigrate` cho production
- GORM tag `type:decimal(x,y)` phải khớp migration
- Sau thêm decimal column vào bảng dùng `map[string]interface{}`: query thủ công `.Find(&rows)` phải gọi `helpers.ParseDecimalFields(row, "field")` thủ công
- Schema gốc: `backend/migrations/<timestamp>_create_<bảng>.up.sql` (không còn file baseline — bản ERP cũ ở `migrations_backup/`, migrate không đọc)

## UNIQUE KEY + NULL column — dùng sentinel 0, không NULL (CRITICAL)

`UNIQUE KEY (template_type, branch_id)` với `branch_id NULL` → MySQL cho phép nhiều row vì `NULL ≠ NULL` → `ON CONFLICT` không trigger → duplicate records âm thầm.

```sql
-- ❌ NEVER
branch_id BIGINT UNSIGNED NULL
UNIQUE KEY uq (template_type, branch_id)  -- nhiều row (X, NULL) được phép

-- ✅ ALWAYS — sentinel 0 = mẫu chung toàn thương hiệu (vd print_templates.branch_id)
branch_id BIGINT UNSIGNED NOT NULL DEFAULT 0
UNIQUE KEY uq (template_type, branch_id)  -- (X, 0) enforce unique
```

**Migration chuyển NULL → sentinel 0:**
```sql
UPDATE table SET branch_id = 0 WHERE branch_id IS NULL;
ALTER TABLE table MODIFY branch_id BIGINT UNSIGNED NOT NULL DEFAULT 0;
-- ADD UNIQUE KEY nếu chưa có
```

**Query:** `WHERE branch_id = 0` thay vì `WHERE branch_id IS NULL`.
**Fallback (branch → mẫu chung toàn thương hiệu; trước D15 gọi là company-level):** `WHERE branch_id IN (?, 0) ORDER BY branch_id DESC LIMIT 1`.

**Ngoại lệ — cột PHẢI giữ NULL (có FOREIGN KEY thật: sentinel 0 vỡ FK vì không có row id=0):** dùng functional key part (MySQL ≥ 8.0.13):
```sql
ALTER TABLE account_balance ADD UNIQUE KEY uq_ab ((COALESCE(customer_id, 0)), account_id, period);
```
Dedupe dữ liệu trước khi ADD — xem section "Đổi unique key trên bảng có dữ liệu". Đã áp (ERP, bảng kế toán — đã bỏ cùng D2/D15): `uq_account_balance_dim0`, `uq_coa_company_code`.

**Chiều NGƯỢC lại — cột CỐ Ý giữ NULL để UNIQUE cho phép nhiều row: phải chuẩn hoá `''` → NULL TRƯỚC khi ADD UNIQUE.** MySQL coi chuỗi rỗng là một GIÁ TRỊ, nên nhiều NULL thì hợp lệ còn nhiều `''` là `Error 1062`. Code ghi NULL không có nghĩa dữ liệu cũ/seed cũng NULL.
```sql
UPDATE t SET col = NULL WHERE col = '';   -- idempotent, chạy TRƯỚC
-- rồi mới ADD UNIQUE KEY (col)
```
Ca biên của một ràng buộc UNIQUE phải phủ **cả hai biểu diễn "không có giá trị"** (`NULL` và `''`) — thử mỗi NULL rồi kết luận an toàn là bỏ lọt. Thật (2026-08-31, `meta_page_config.ig_business_account_id`): verify chỉ thử NULL nên pass, tới khi test khác seed `''` mới lộ; nếu lọt lên DB khách thì migration fail và chặn cả đợt deploy.

---

## Atomic counter upsert — cột cộng dồn (balance/counter)

- KHÔNG `First → Save` cho cột cộng dồn — read-modify-write bị lost-update khi 2 request song song. Chuẩn: UPDATE tăng trực tiếp → `RowsAffected == 0` → INSERT → INSERT dính 1062 (thua đua) → retry UPDATE lần cuối.
- MySQL đánh giá `SET` **trái→phải**: cột dẫn xuất đặt SAU dùng giá trị MỚI của các cột trước.

```sql
UPDATE account_balance
SET debit_total = debit_total + ?, credit_total = credit_total + ?,
    closing = IF(? = 2, credit_total - debit_total, debit_total - credit_total)  -- đã là giá trị MỚI
WHERE ...;
```

- Nhánh INSERT cần unique index backstop (cột nullable → functional key part, xem section trên). Mẫu Go: `engine.ApplyBalanceDelta` (`accountant/ledger/engine`).

---

## GORM Raw Query — args ordering (CRITICAL)

`?` placeholder được bind **theo thứ tự xuất hiện trong SQL string, từ trên xuống, trái sang phải** — kể cả trong SELECT clause.

```go
// ❌ SAI — tưởng WHERE trước, CASE WHEN sau
args = append(args, consultTypeIDs, from, to, CheckedIn, CheckedOut, IsAccepted)

// ✅ ĐÚNG — CASE WHEN trong SELECT xuất hiện TRƯỚC WHERE trong SQL
args = append(args,
    CheckedIn, CheckedOut,  // CASE WHEN a.status IN (?,?)     ← ở SELECT
    IsAccepted,             // CASE WHEN a.is_accepted = ?      ← ở SELECT
    consultTypeIDs, from, to) // WHERE type IN ?, BETWEEN ? AND ? ← ở WHERE
```

- Subquery lồng nhau: args của subquery phía trên được bind trước subquery phía dưới
- `IN ?` với `[]int` slice: GORM tự expand thành `(1,2,3)` — không cần làm thủ công
- `IN ?` với empty slice `[]int{}`: GORM sinh `IN (NULL)` — query không lỗi nhưng không match row nào

## Đổi unique key trên bảng có dữ liệu

- golang-migrate chạy MỌI migration pending liên tiếp trong 1 lần `-cmd=up` — KHÔNG thiết kế "migration A → script ngoài → migration B"; backfill bằng SQL thuần (`REPLACE`/`UPPER` lồng nhau đủ mạnh) trong CÙNG file.
- Trước `ADD UNIQUE KEY`: bắt buộc dedupe dữ liệu hiện có bằng SQL (giữ `MIN(id)`, row trùng đổi giá trị hậu tố `_DUP<id>` — không xóa cứng); bỏ sót → constraint fail → dirty state block deploy.

## (đã bỏ, D15) Seed bảng scope theo company — per-company, không anchor 1 company duy nhất

> Lịch sử: bảng `company` + cột `company_id` không còn (1 thương hiệu/DB). Seed nay chỉ cần idempotent
> `WHERE NOT EXISTS`/`INSERT IGNORE` trong phạm vi 1 DB (Hard Rules); bài học còn giá trị: đừng
> neo seed vào 1 dòng `MIN(id)` của bảng cha khi bảng cha có thể có nhiều dòng (vd `branch`).

(Lịch sử, D15) Seed `config_code` (hay bảng nào cần 1 row/`company_id`) KHÔNG dùng `SELECT MIN(id) FROM company` làm anchor — chỉ seed đúng 1 company, DB nhiều company (multi-brand cùng 1 khách) sẽ có company thiếu row, rơi vào fallback "bất kể company" sai tenant.

```sql
-- (lịch sử, D15) ❌ SAI — chỉ seed company nhỏ nhất
INSERT INTO config_code (company_id, ...) SELECT (SELECT MIN(id) FROM company), ...  -- (lịch sử, D15)
WHERE NOT EXISTS (SELECT 1 FROM config_code WHERE parent_table = 'service');

-- ✅ ĐÚNG — seed đủ mọi company
INSERT INTO config_code (company_id, ...)  -- (lịch sử, D15)
SELECT c.id, ... FROM company c
WHERE NOT EXISTS (SELECT 1 FROM config_code WHERE parent_table = 'service' AND company_id = c.id);  -- (lịch sử, D15)
```
- Cột nhận hậu tố dedupe phải khai đủ rộng (vd VARCHAR(20) thay vì VARCHAR(10)) — strict mode nổ `Data too long` ngay trong migration.

## Kiểm "đã seed chưa" — dò theo KHOÁ SEED, KHÔNG theo `id`

Audit "record X đã có migration seed chưa" phải dò theo **khoá seed thật** (`name`, `key`, `handler_key`), KHÔNG theo `id`.
`INSERT IGNORE INTO t (name, ...)` không có cột `id` ⇒ mọi phép dò dựa trên `id` **mù hoàn toàn**: con số id không xuất hiện ở bất kỳ đâu trong file migration.
`id` là `AUTO_INCREMENT` ⇒ **không ổn định giữa các DB khách** — cùng câu seed cho id 41 ở DB này, 45 ở DB khác.
`INSERT IGNORE` trùng khoá **im lặng không làm gì** ⇒ "chạy 2 lượt mà giá trị không đổi" KHÔNG phân biệt được *idempotent* với *câu lệnh chưa bao giờ chạy*; xoá dòng rồi chạy lại để chứng minh lệnh có tác dụng.

## Data-fix cột JSON array — additive, idempotent

Thêm phần tử vào cột JSON array (vd `departmentList`) để gỡ chặn nghiệp vụ: dùng
`JSON_ARRAY_APPEND` bọc `NOT JSON_CONTAINS` — chạy lại không double-add, không đụng phần tử cũ.

```sql
UPDATE t SET col = JSON_ARRAY_APPEND(col, '$', <value>)
WHERE id = ? AND NOT JSON_CONTAINS(col, '<value>');
```

## UNION ALL bảng cũ + bảng mới → MySQL 1271 "Illegal mix of collations"

Bảng cũ không khai `COLLATE` tường minh kế thừa collation mặc định server; bảng mới khai rõ
(vd `utf8mb4_unicode_ci`) → UNION ALL 2 nhánh lẫn collation khác nhau, lỗi CHỈ lộ khi chạy
query thật (không phải lỗi Go/build). Fix: ép `COLLATE utf8mb4_unicode_ci` tường minh trên
MỌI cột string (kể cả literal) ở TẤT CẢ nhánh UNION, không chỉ nhánh mới.

KHÔNG `SET col = '[...]'` ghi đè nguyên mảng — mất phần tử cũ nếu tính base sai.

## Cột SINH (generated column) + khoá ngoại: phải VIRTUAL, KHÔNG STORED

**Đo thật trên MySQL 9.7.1 (2026-08-25, `omi_message_reaction`):** MySQL từ chối **MỌI** `FOREIGN KEY`
đặt trên **cột nền** của một generated column **STORED** — thử đủ 3 tổ hợp
`ON DELETE SET NULL/CASCADE` × `ON UPDATE CASCADE/RESTRICT`, tất cả đều `ERROR 1215 (HY000): Cannot
add foreign key constraint`. Cùng biểu thức đó để **VIRTUAL** thì FK tạo được bình thường, và UNIQUE
trên cột ảo VẪN chặn trùng thật (`ERROR 1062`) — InnoDB cho phép index phụ trên cột ảo.

Vì sao dễ mất thời gian ở đây: lỗi 1215 **không nói cột nào gây ra**, và `SHOW ENGINE INNODB STATUS`
trên MySQL 9.x không còn khối `LATEST FOREIGN KEY ERROR`. Cách chẩn đoán nhanh: tạo bảng thử với
từng FK riêng lẻ (đều chạy được) rồi thêm cột sinh vào → lộ ngay thủ phạm.

**Khi nào cần cột sinh:** MySQL KHÔNG có partial/filtered index. Muốn một `UNIQUE` phủ hai loại chủ
thể loại trừ nhau (vd người thả cảm xúc: nhân viên nội bộ có FK `user_id` ↔ khách chỉ có chuỗi uid),
hội tụ chúng về một cột sinh rồi UNIQUE trên đó:
`reactor_key VARCHAR(120) GENERATED ALWAYS AS (IF(user_id IS NULL, channel_uid, CONCAT('u:', user_id))) VIRTUAL`
⚠ **Tiền tố (`u:`) là bắt buộc** — thiếu nó thì `user_id = 123` và `channel_uid = "123"` đụng nhau.

Entity GORM: cột sinh phải `gorm:"->"` (chỉ đọc). Thiếu tag ⇒ GORM đưa cột vào INSERT ⇒ **mọi**
`Create` lỗi, và chỉ lộ ra khi có code thật dùng entity (tức là muộn).


## ⚠ Ghi file vào `migrations/` = ĐÃ APPLY lên DB dev trong vài giây (đo 2026-09-08)

`backend/scripts/run_dev.sh` chạy `migrate up` **mỗi lần air rebuild**, và air rebuild ngay khi có
file mới. Nghĩa là: **không có bước "viết xong rồi mới quyết định chạy"** — vừa `Write` file
`.up.sql` là nó đã chạy trên DB dev thật, trước khi kịp verify trên DB dùng-một-lần.

Bằng chứng (`backend/tmp/run.log`):

```
[16:33:36] run_dev.sh: migrate OK — migrate up: không có migration mới (version 20260908155444)
[16:36:43] run_dev.sh: migrate OK — migrate up: 20260908155444 → 20260908163530 thành công
```

File `20260908163530_*.up.sql` được ghi lúc 16:36 và tự apply lúc 16:36:44.

Hệ quả bắt buộc nhớ:

1. **Verify TRƯỚC khi ghi vào `migrations/`.** Soạn nội dung ở scratchpad của phiên, chạy thử trên
   DB `*_tmp` (2 lần cho idempotency + 1 lần trên DB có dữ liệu vi phạm để xem cổng chặn có fail
   thật không), rồi mới `Write` vào `migrations/`. Làm ngược lại thì DB dev là nơi thử nghiệm đầu
   tiên — đúng thứ luật ⛔ đầu file này cấm.
2. **Migration hỏng ⇒ `dirty=1` và dev server của user chết** cho tới khi có người dọn tay. Kiểm
   `SELECT dirty, version FROM schema_migrations` sau khi ghi file.
3. Sửa nội dung một migration ĐÃ apply thì DB dev **không chạy lại** (golang-migrate chỉ nhớ
   version, không có checksum) — nhưng DB khách khác sẽ nhận bản MỚI. Chỉ sửa khi nội dung mới
   tương đương về ngữ nghĩa, và phải verify lại bản mới trên DB dùng-một-lần.

## Rẽ nhánh có điều kiện: cách bắt migration DỪNG với thông báo đọc được

Cổng kiểm dữ liệu (vd "còn bản sao thì đừng thêm UNIQUE") mà chỉ `IF NOT EXISTS` rồi im lặng không
tạo sẽ báo **thành công trên DB vẫn còn lỗ hổng** — xanh giả. Phải fail thật.

- `SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = '…'` cho thông báo đẹp nhất và **chạy được ngoài
  stored program**, NHƯNG **không dùng được ở đây**: `ERROR 1295 … not supported in the prepared
  statement protocol`. Mà `PREPARE`/`EXECUTE` lại là cách duy nhất để rẽ nhánh có điều kiện trong
  file `.sql` không có stored program. Đã đo cả hai chiều 2026-09-08.
- Cách còn lại: tham chiếu **một bảng không tồn tại mà TÊN chính là thông báo**:
  `'SELECT 1 FROM \`MIGRATION_ABORT_duplicate_customer_id_in_customer_warranty_code\`'`
  ⇒ `ERROR 1146 … Table 'db.migration_abort_duplicate_customer_id_in_customer_warranty_code'
  doesn't exist`, exit code 1, và các câu lệnh sau KHÔNG chạy. Tên ≤64 ký tự (giới hạn identifier).

⚠ Khi kiểm exit code trong shell: `mysql … | grep -v "Using a password"` trả exit code của **grep**,
không phải của mysql. Đo đúng phải bỏ pipe (`mysql … > /dev/null 2>&1; echo $?`) hoặc đọc
`PIPESTATUS`. Đã suýt kết luận "cổng chặn không fail" vì đọc nhầm số 0 của grep.

⚠ Cùng nguyên nhân, hậu quả khác: `mysql … | grep -v "Using a password" && <lệnh kế>` — không dòng
nào lọt qua grep ⇒ exit 1 ⇒ **lệnh kế bị bỏ qua im lặng**. Nối nhiều bước DB thì tách từng lệnh
riêng, và luôn `SELECT COUNT(*)` xác nhận bước trước đã ghi được dữ liệu.

## Thứ tự bắt buộc khi đổi chỉ mục mà FK đang dựa vào

`ALTER TABLE … DROP INDEX` trên cột có FOREIGN KEY sẽ bị MySQL từ chối
(`ERROR 1553 … needed in a foreign key constraint`) nếu không còn chỉ mục nào khác đỡ FK. Nên khi
thay chỉ mục thường bằng UNIQUE trên CÙNG cột: **ADD UNIQUE trước, DROP index thường sau** — thứ tự
ngược lại fail. Kiểm sau khi chạy: FK phải còn trong `information_schema.KEY_COLUMN_USAGE`.
