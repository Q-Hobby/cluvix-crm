#!/usr/bin/env python3
"""collation-scan.py — lưới chống tái sinh lỗi LẪN COLLATION.

Chuẩn dự án: MỌI bảng dùng `utf8mb4` / `utf8mb4_unicode_ci`. Bảng lệch chuẩn gây lỗi runtime khi
so sánh chuỗi với bảng đúng chuẩn:
    JOIN  → ERROR 1267 "Illegal mix of collations ... for operation '='"
    UNION → ERROR 1271 "Illegal mix of collations for operation 'UNION'"
Lỗi CHỈ nổ lúc chạy query cụ thể — `go build` xanh, `migrate up` xanh, không ai biết cho tới khi
người dùng bấm đúng chức năng. Đã cắn dự án 1 lần: `webapp/emr/patient_document/handler/summary.go`
phải ép `COLLATE` thủ công lên từng cột string của 4 nhánh UNION.

Rule tương ứng: `skill/mysql/prompts/migration.md` (template CREATE TABLE) + `skill/mysql/mysql97.md` §9.
Kế hoạch xử lý nợ hiện có: `skill/bmad/mysql97-backend-alignment/`.

HAI CHẾ ĐỘ ĐỘC LẬP — cho 2 câu hỏi KHÁC NHAU, kết quả KHÔNG bắt buộc trùng nhau:
    --static  (mặc định, không cần DB)  "DB dựng MỚI từ migrations có ra đúng collation không?"
    --live    (cần DB)                  "DB ĐANG CHẠY hiện lệch những bảng nào?"

⚠ VÌ SAO 2 DANH SÁCH KHÁC NHAU LÀ ĐÚNG, KHÔNG PHẢI BUG:
    Sửa file migration KHÔNG sửa bảng đã apply. Bằng chứng thật:
    `20260415141827_create_bank_accounts.up.sql` ghi ĐÚNG `COLLATE=utf8mb4_unicode_ci` (file được sửa
    sau khi đã apply) nhưng bảng `bank_accounts` trong DB dev vẫn là `utf8mb4_0900_ai_ci`.
    → static SẠCH cho file đó, live VẪN BÁO bảng đó. Đừng "sửa" script cho 2 danh sách bằng nhau.

⚠ ĐỌC TRƯỚC KHI TIN KẾT QUẢ — vì sao có SELF_TEST:
    Lưới an toàn nguy hiểm nhất là lưới luôn xanh. Parser SQL viết ẩu (bắt theo từng dòng thay vì
    theo statement) sẽ bỏ sót `CREATE TABLE` có `COLLATE` nằm khác dòng, rồi báo "sạch" trong khi
    schema vẫn lệch. Script LUÔN tự kiểm trên fixture có cả ca DƯƠNG lẫn ca ÂM trước khi quét thật;
    self-test fail ⇒ exit 2, KHÔNG in kết quả.
    Self-test dùng FIXTURE nhúng, không dùng số file thật trong repo — để nó vẫn đúng SAU KHI
    story-01 vá xong 10 file (số thật sẽ về 0, fixture thì không đổi).

Exit code: 0 = sạch · 1 = có finding · 2 = self-test fail (script hỏng) · 3 = lỗi vận hành.

Dùng:
    python3 scripts/collation-scan.py                      # static, mặc định
    python3 scripts/collation-scan.py --static
    python3 scripts/collation-scan.py --live cluvix_erp --user root
    MYSQL_PWD=xxx python3 scripts/collation-scan.py --live cluvix_erp_tmp --user root
"""
import argparse
import os
import re
import subprocess
import sys

BE = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
MIGRATIONS = os.path.join(BE, 'migrations')

STANDARD_COLLATION = 'utf8mb4_unicode_ci'
STANDARD_CHARSET = 'utf8mb4'

# Baseline là ảnh chụp schema kế thừa, không phải migration do dự án viết — quét nó chỉ tạo nhiễu.
SKIP_FILES = {'00000000000000_baseline.up.sql'}

# Bảng hạ tầng, không JOIN với bảng nghiệp vụ nào → miễn trừ khỏi exit code (AD2 của architecture.md).
# `schema_migrations` do golang-migrate quản lý; `migration` là bảng legacy Yii không code Go nào đọc.
EXEMPT_TABLES = {'schema_migrations', 'migration'}

CREATE_TABLE_RE = re.compile(
    r'CREATE\s+(?:TEMPORARY\s+)?TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?[`"]?(\w+)[`"]?',
    re.I,
)
COLLATE_RE = re.compile(r'COLLATE\s*=?\s*[`"]?(\w+)[`"]?', re.I)
CHARSET_RE = re.compile(r'(?:DEFAULT\s+)?(?:CHARACTER\s+SET|CHARSET)\s*=?\s*[`"]?(\w+)[`"]?', re.I)


def strip_sql_comments(sql):
    """Bỏ comment VÀ nội dung chuỗi nháy đơn — để không bắt nhầm `CREATE TABLE` nằm trong đó.

    Nháy đơn phải blank vì migration dạng dynamic SQL (`SET @s = 'CREATE TABLE ...'; PREPARE ...`)
    sẽ tạo false positive — và `migration-idempotent.md` nay KHUYẾN KHÍCH dạng này.
    Backtick thì KHÔNG blank: nó bọc tên bảng, blank đi là mất luôn tên cần bắt.
    Nháy kép cũng không blank (sql_mode mặc định không dùng nó làm identifier, nhưng blank thì rủi ro
    hơn lợi — dynamic SQL trong repo này viết bằng nháy đơn).

    Giữ nguyên độ dài chuỗi (thay bằng khoảng trắng) để offset → số dòng vẫn đúng.
    """
    out = list(sql)
    i, n = 0, len(sql)
    in_line, in_block, quote = False, False, None
    while i < n:
        ch = sql[i]
        nxt = sql[i + 1] if i + 1 < n else ''
        if in_line:
            if ch == '\n':
                in_line = False
            else:
                out[i] = ' '
        elif in_block:
            if ch == '*' and nxt == '/':
                out[i] = out[i + 1] = ' '
                i += 2
                continue
            if ch != '\n':
                out[i] = ' '
        elif quote:
            if ch == '\\':
                if quote == "'" and i + 1 < n:
                    out[i] = out[i + 1] = ' '
                i += 2
                continue
            if ch == quote:
                quote = None
            elif quote == "'" and ch != '\n':
                out[i] = ' '
        else:
            if ch == '-' and nxt == '-':
                in_line = True
                out[i] = ' '
            elif ch == '#':
                in_line = True
                out[i] = ' '
            elif ch == '/' and nxt == '*':
                in_block = True
                out[i] = out[i + 1] = ' '
                i += 2
                continue
            elif ch in ("'", '"', '`'):
                quote = ch
        i += 1
    return ''.join(out)


def table_options_after_body(sql, start):
    """Trả (options, end_index) — phần table_options nằm SAU thân ngoặc của CREATE TABLE.

    Bắt theo ĐỘ SÂU NGOẶC, không theo dòng: `COLLATE` hợp lệ có thể nằm cùng dòng `) ENGINE=...`
    hoặc xuống dòng riêng. Trả (None, ...) nếu không có thân ngoặc (CREATE TABLE ... LIKE / AS SELECT).
    """
    i, n = start, len(sql)
    while i < n and sql[i] != '(':
        # Gặp `;` trước `(` → dạng LIKE / AS SELECT, không có thân ngoặc.
        if sql[i] == ';':
            return None, i
        i += 1
    if i >= n:
        return None, n
    depth, i = 0, i
    while i < n:
        if sql[i] == '(':
            depth += 1
        elif sql[i] == ')':
            depth -= 1
            if depth == 0:
                i += 1
                break
        i += 1
    end = sql.find(';', i)
    if end == -1:
        end = n
    return sql[i:end], end


def scan_sql_text(sql, origin='<text>'):
    """Quét 1 nội dung SQL → list finding dict. Dùng chung cho file thật và fixture self-test."""
    clean = strip_sql_comments(sql)
    findings = []
    for m in CREATE_TABLE_RE.finditer(clean):
        table = m.group(1)
        opts, _ = table_options_after_body(clean, m.end())
        line = clean.count('\n', 0, m.start()) + 1
        if opts is None:
            findings.append({
                'file': origin, 'line': line, 'table': table,
                'reason': 'CREATE TABLE ... LIKE/AS SELECT — kế thừa collation bảng nguồn, kiểm tay',
                'kind': 'unknown',
            })
            continue
        col = COLLATE_RE.search(opts)
        cs = CHARSET_RE.search(opts)
        if col is None:
            # Hai đường thừa kế KHÁC NHAU (đã verify thực nghiệm trên 9.7.1):
            #   có CHARSET, không COLLATE → default của CHARSET đó  → utf8mb4_0900_ai_ci
            #   không có mệnh đề charset  → default của DATABASE    → tuỳ DB (cluvix_erp: general_ci)
            if cs is None:
                reason = 'thiếu cả CHARSET lẫn COLLATE → kế thừa default của DATABASE (tuỳ từng DB khách)'
            elif cs.group(1).lower() == STANDARD_CHARSET:
                reason = 'thiếu COLLATE → kế thừa default của charset utf8mb4 = utf8mb4_0900_ai_ci'
            else:
                reason = 'thiếu COLLATE, charset=%s' % cs.group(1)
            findings.append({'file': origin, 'line': line, 'table': table,
                             'reason': reason, 'kind': 'missing'})
        elif col.group(1).lower() != STANDARD_COLLATION:
            findings.append({'file': origin, 'line': line, 'table': table,
                             'reason': 'COLLATE=%s (chuẩn: %s)' % (col.group(1), STANDARD_COLLATION),
                             'kind': 'wrong'})
        elif cs and cs.group(1).lower() != STANDARD_CHARSET:
            findings.append({'file': origin, 'line': line, 'table': table,
                             'reason': 'CHARSET=%s (chuẩn: %s)' % (cs.group(1), STANDARD_CHARSET),
                             'kind': 'wrong'})
    return findings


# --- SELF-TEST --------------------------------------------------------------
# Ca ÂM (phải KHÔNG báo) và ca DƯƠNG (phải báo). Fixture nhúng → không phụ thuộc trạng thái repo.
SELF_TEST_CASES = [
    # (nhãn, sql, số finding mong đợi)
    ('đúng chuẩn, 1 dòng',
     "CREATE TABLE `a` (`id` int) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;", 0),
    ('đúng chuẩn, COLLATE xuống dòng riêng',
     "CREATE TABLE `a` (\n `id` int\n) ENGINE=InnoDB\n  DEFAULT CHARSET=utf8mb4\n  COLLATE=utf8mb4_unicode_ci;", 0),
    ('đúng chuẩn, có COMMENT sau COLLATE',
     "CREATE TABLE `a` (`id` int) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='x';", 0),
    ('THIẾU COLLATE (có CHARSET)',
     "CREATE TABLE `a` (`id` int) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;", 1),
    ('THIẾU cả CHARSET lẫn ENGINE — kế thừa default của DATABASE',
     "CREATE TABLE IF NOT EXISTS hr_x (\n  id int NOT NULL AUTO_INCREMENT,\n  PRIMARY KEY (id)\n);", 1),
    ('CHARACTER SET viết đầy đủ (không phải CHARSET=) vẫn nhận ra',
     "CREATE TABLE `a` (`id` int) ENGINE=InnoDB DEFAULT CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci;", 0),
    ('SAI COLLATE (general_ci)',
     "CREATE TABLE `a` (`id` int) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;", 1),
    ('SAI charset (utf8mb3)',
     "CREATE TABLE `a` (`id` int) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3 COLLATE=utf8mb3_unicode_ci;", 1),
    ('COLLATE của CỘT không được tính là của BẢNG',
     "CREATE TABLE `a` (`n` varchar(9) COLLATE utf8mb4_unicode_ci) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;", 1),
    ('ngoặc lồng (KEY nhiều cột, functional index) không phá parser',
     "CREATE TABLE `a` (`id` int, `k` varchar(9), UNIQUE KEY `u` ((IF(`id`=1,`k`,NULL))), KEY `x` (`id`,`k`))"
     " ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;", 0),
    ('CREATE TABLE trong comment bị bỏ qua',
     "-- CREATE TABLE `ghost` (`id` int) ENGINE=InnoDB;\nSELECT 1;", 0),
    ('CREATE TABLE trong CHUỖI NHÁY ĐƠN (dynamic SQL) bị bỏ qua',
     "SET @s = 'CREATE TABLE t (id int) ENGINE=InnoDB'; PREPARE st FROM @s;", 0),
    ('dynamic SQL nhiều dòng không nuốt mất CREATE TABLE thật phía sau',
     "SET @s = 'ALTER TABLE x ADD COLUMN y int';\nPREPARE st FROM @s;\n"
     "CREATE TABLE `real` (`id` int) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;", 1),
    ('COMMENT có nháy đơn không phá việc đọc COLLATE',
     "CREATE TABLE `a` (`id` int) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4"
     " COLLATE=utf8mb4_unicode_ci COMMENT='ghi chú có CREATE TABLE trong đó';", 0),
    ('2 CREATE TABLE trong 1 file, 1 sai',
     "CREATE TABLE `a` (`id` int) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;\n"
     "CREATE TABLE `b` (`id` int) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;", 1),
    ('IF NOT EXISTS vẫn bắt được',
     "CREATE TABLE IF NOT EXISTS `a` (`id` int) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;", 1),
]


def self_test():
    failed = []
    for label, sql, expect in SELF_TEST_CASES:
        got = len(scan_sql_text(sql, '<self-test>'))
        if got != expect:
            failed.append('  ✗ %s — mong đợi %d finding, nhận %d' % (label, expect, got))
    if failed:
        print('SELF-TEST FAIL — script hỏng, KHÔNG tin kết quả quét:', file=sys.stderr)
        print('\n'.join(failed), file=sys.stderr)
        return False
    return True


# --- STATIC -----------------------------------------------------------------
def run_static():
    if not os.path.isdir(MIGRATIONS):
        print('Không thấy thư mục migrations: %s' % MIGRATIONS, file=sys.stderr)
        return 3
    findings = []
    files = sorted(f for f in os.listdir(MIGRATIONS) if f.endswith('.up.sql'))
    scanned = 0
    for name in files:
        if name in SKIP_FILES:
            continue
        scanned += 1
        with open(os.path.join(MIGRATIONS, name), encoding='utf-8', errors='replace') as fh:
            findings.extend(scan_sql_text(fh.read(), name))

    print('== STATIC: CREATE TABLE trong migrations thiếu/sai COLLATE ==')
    print('   quét %d file (.up.sql), bỏ qua baseline\n' % scanned)
    if not findings:
        print('   sạch — mọi CREATE TABLE đều khai COLLATE=%s' % STANDARD_COLLATION)
        return 0
    by_file = {}
    for f in findings:
        by_file.setdefault(f['file'], []).append(f)
    for name in sorted(by_file):
        print('  %s' % name)
        for f in by_file[name]:
            print('      dòng %-5d %-32s %s' % (f['line'], f['table'], f['reason']))
    print('\n   %d bảng / %d file' % (len(findings), len(by_file)))
    print('   Vá: thêm `COLLATE=utf8mb4_unicode_ci` vào mệnh đề ENGINE=InnoDB DEFAULT CHARSET=utf8mb4.')
    print('   LƯU Ý: sửa file KHÔNG đổi bảng đã apply — cần thêm migration ALTER (xem story-01).')
    return 1


# --- LIVE -------------------------------------------------------------------
def mysql_query(args, sql):
    cmd = ['mysql', '--batch', '--raw', '--skip-column-names',
           '-h', args.host, '-P', str(args.port), '-u', args.user, '-e', sql]
    env = dict(os.environ)
    if args.password is not None:
        env['MYSQL_PWD'] = args.password  # qua env, không lộ trên `ps`
    try:
        out = subprocess.run(cmd, capture_output=True, text=True, env=env, timeout=60)
    except FileNotFoundError:
        print('Không tìm thấy lệnh `mysql` trong PATH.', file=sys.stderr)
        sys.exit(3)
    except subprocess.TimeoutExpired:
        print('Query quá 60s — bỏ.', file=sys.stderr)
        sys.exit(3)
    err = '\n'.join(l for l in out.stderr.splitlines() if 'Using a password' not in l).strip()
    if out.returncode != 0:
        print('mysql lỗi: %s' % (err or out.stdout.strip()), file=sys.stderr)
        sys.exit(3)
    if err:
        print('cảnh báo mysql: %s' % err, file=sys.stderr)
    return [line.split('\t') for line in out.stdout.splitlines() if line.strip()]


Q_SCHEMA_DEFAULT = """
SELECT DEFAULT_CHARACTER_SET_NAME, DEFAULT_COLLATION_NAME
FROM information_schema.SCHEMATA WHERE SCHEMA_NAME = '{db}';
"""

Q_DRIFT = """
SELECT TABLE_NAME, TABLE_COLLATION, IFNULL(TABLE_ROWS,0), IFNULL(DATA_LENGTH,0)
FROM information_schema.TABLES
WHERE TABLE_SCHEMA = '{db}' AND TABLE_TYPE = 'BASE TABLE'
  AND TABLE_COLLATION <> '{coll}'
ORDER BY TABLE_COLLATION, TABLE_NAME;
"""

Q_UNIQUE_STR = """
SELECT s.TABLE_NAME, s.INDEX_NAME, s.COLUMN_NAME, IFNULL(t.TABLE_ROWS,0)
FROM information_schema.STATISTICS s
JOIN information_schema.COLUMNS c ON c.TABLE_SCHEMA=s.TABLE_SCHEMA AND c.TABLE_NAME=s.TABLE_NAME
 AND c.COLUMN_NAME=s.COLUMN_NAME
JOIN information_schema.TABLES t ON t.TABLE_SCHEMA=s.TABLE_SCHEMA AND t.TABLE_NAME=s.TABLE_NAME
WHERE s.TABLE_SCHEMA = '{db}' AND s.NON_UNIQUE = 0 AND c.COLLATION_NAME IS NOT NULL
  AND t.TABLE_COLLATION <> '{coll}'
ORDER BY s.TABLE_NAME, s.INDEX_NAME;
"""

Q_FK_STR = """
SELECT k.TABLE_NAME, k.CONSTRAINT_NAME, k.COLUMN_NAME,
       CONCAT(k.REFERENCED_TABLE_NAME,'.',k.REFERENCED_COLUMN_NAME)
FROM information_schema.KEY_COLUMN_USAGE k
JOIN information_schema.COLUMNS c ON c.TABLE_SCHEMA=k.TABLE_SCHEMA AND c.TABLE_NAME=k.TABLE_NAME
 AND c.COLUMN_NAME=k.COLUMN_NAME
WHERE k.TABLE_SCHEMA = '{db}' AND k.REFERENCED_TABLE_NAME IS NOT NULL
  AND c.COLLATION_NAME IS NOT NULL
ORDER BY k.TABLE_NAME;
"""


def run_live(args):
    db = args.live
    env_db = read_env_db_name()
    if env_db and db == env_db:
        print('⚠  CẢNH BÁO: `%s` là DB_NAME trong backend/.env (DB dev/prod thật).' % db)
        print('   Script này CHỈ ĐỌC nên an toàn, nhưng migration/test thì TUYỆT ĐỐI không trỏ vào đây.')
        print('   Xem `skill/mysql/prompts/migration.md` mục ⛔.\n')

    # DB không tồn tại → information_schema trả RỖNG, không phải lỗi. Không chặn ở đây thì script
    # báo "sạch" cho một cái tên gõ sai — đúng kiểu lưới-luôn-xanh mà docblock cảnh báo.
    schema_rows = mysql_query(args, Q_SCHEMA_DEFAULT.format(db=db))
    if not schema_rows:
        print('Database `%s` không tồn tại (hoặc user `%s` không thấy được nó).'
              % (db, args.user), file=sys.stderr)
        sys.exit(3)

    rows = mysql_query(args, Q_DRIFT.format(db=db, coll=STANDARD_COLLATION))
    drift = [r for r in rows if r[0] not in EXEMPT_TABLES]
    exempt = [r for r in rows if r[0] in EXEMPT_TABLES]

    print('== LIVE: bảng lệch collation trong `%s` ==\n' % db)

    # (0) Default của DATABASE — gốc rò rỉ. CREATE TABLE không khai mệnh đề charset sẽ kế thừa cái này.
    db_default = schema_rows[0][1]
    print('-- (0) default collation của DATABASE --')
    if db_default == STANDARD_COLLATION:
        print('   %s — đúng chuẩn' % db_default)
    else:
        print('   %s  ⚠ LỆCH CHUẨN (%s)' % (db_default, STANDARD_COLLATION))
        print('   Bảng tạo mới KHÔNG khai mệnh đề charset sẽ kế thừa giá trị này → rò rỉ tiếp tục.')
        print('   Vá gốc:  ALTER DATABASE `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;' % db)
        print('   (chỉ đổi default cho bảng TẠO SAU — không đụng bảng đã có)')
    print('')
    print('-- (a) bảng khác %s --' % STANDARD_COLLATION)
    if not drift:
        print('   sạch')
    else:
        print('   %-34s %-22s %10s %12s' % ('BẢNG', 'COLLATION', 'ROWS', 'DATA_BYTES'))
        for name, coll, nrows, dlen in drift:
            print('   %-34s %-22s %10s %12s' % (name, coll, nrows, dlen))
        big = [r for r in drift if int(r[3] or 0) > 512 * 1024 * 1024]
        if big:
            print('\n   ⚠ bảng >512MB (CONVERT TO rebuild toàn bảng → cân nhắc cửa sổ downtime): %s'
                  % ', '.join(r[0] for r in big))
    if exempt:
        print('\n   miễn trừ (không tính vào exit code): %s'
              % ', '.join('%s [%s]' % (r[0], r[1]) for r in exempt))

    drift_names = {r[0] for r in drift}

    print('\n-- (b) UNIQUE trên cột chuỗi thuộc nhóm (a) — rủi ro R7 --')
    print('   Đổi collation có thể làm 2 giá trị đang KHÁC nhau thành TRÙNG → ALTER fail vì trùng khoá.')
    uq = [r for r in mysql_query(args, Q_UNIQUE_STR.format(db=db, coll=STANDARD_COLLATION))
          if r[0] in drift_names]
    if not uq:
        print('   không có')
    else:
        for tbl, idx, col, nrows in uq:
            flag = '' if nrows == '0' else '   ← có dữ liệu, dedupe trước khi convert'
            print('   %-34s %-24s %-20s rows=%s%s' % (tbl, idx, col, nrows, flag))

    print('\n-- (c) FOREIGN KEY trên cột chuỗi — rủi ro R2 (Error 3780) --')
    print('   CONVERT trần trên bảng cha sẽ FAIL; phải DROP FK → CONVERT → ADD FK lại.')
    fks = [r for r in mysql_query(args, Q_FK_STR.format(db=db))
           if r[0] in drift_names or r[3].split('.')[0] in drift_names]
    if not fks:
        print('   không có')
    else:
        for tbl, cons, col, ref in fks:
            print('   %-24s %-30s %-14s → %s' % (tbl, cons, col, ref))

    print('')
    if drift:
        print('   %d bảng cần convert. Kế hoạch: skill/bmad/mysql97-backend-alignment/' % len(drift))
        return 1
    print('   sạch — mọi bảng nghiệp vụ đã ở %s' % STANDARD_COLLATION)
    return 0


def read_env_db_name():
    """Đọc DB_NAME trong backend/.env — CHỈ để cảnh báo, không dùng làm target mặc định."""
    path = os.path.join(BE, '.env')
    if not os.path.isfile(path):
        return None
    try:
        with open(path, encoding='utf-8', errors='replace') as fh:
            for line in fh:
                line = line.strip()
                if line.startswith('DB_NAME='):
                    return line.split('=', 1)[1].strip().strip('"\'')
    except OSError:
        return None
    return None


def main():
    p = argparse.ArgumentParser(
        description='Quét collation lệch chuẩn utf8mb4_unicode_ci (migrations và/hoặc DB đang chạy).',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog='Exit: 0 sạch · 1 có finding · 2 self-test fail · 3 lỗi vận hành')
    p.add_argument('--static', action='store_true', help='quét backend/migrations/*.up.sql (mặc định)')
    p.add_argument('--live', metavar='DBNAME',
                   help='quét information_schema của DB này (phải ghi tên tường minh)')
    p.add_argument('--host', default='127.0.0.1')
    p.add_argument('--port', default=3306, type=int)
    p.add_argument('--user', default='root')
    p.add_argument('--password', default=os.environ.get('MYSQL_PWD'),
                   help='mặc định lấy từ biến môi trường MYSQL_PWD')
    args = p.parse_args()

    if not self_test():
        return 2

    codes = []
    if args.live:
        codes.append(run_live(args))
    if args.static or not args.live:
        if codes:
            print('')
        codes.append(run_static())
    # Thứ tự ưu tiên: lỗi vận hành (3) > có finding (1) > sạch (0).
    # KHÔNG được để 3 rơi xuống 0 — "không quét được" mà báo "sạch" là lưới luôn xanh.
    if 3 in codes:
        return 3
    return 1 if 1 in codes else 0


if __name__ == '__main__':
    sys.exit(main())
