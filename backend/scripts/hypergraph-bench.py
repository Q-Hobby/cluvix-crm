#!/usr/bin/env python3
"""hypergraph-bench.py — đo plan/thời gian query với và không có hypergraph optimizer (MySQL 9.7+).

CÔNG CỤ CHẨN ĐOÁN THỦ CÔNG. KHÔNG phải thứ để bật trong sản phẩm.

⛔ KHÔNG BAO GIỜ `SET GLOBAL optimizer_switch='hypergraph_optimizer=on'`, không bật trong my.cnf,
   không gọi từ code Go. Lý do:
     - hypergraph BỎ QUA mọi optimizer hint trừ STRAIGHT_JOIN
     - hypergraph KHÔNG hỗ trợ EXPLAIN FORMAT=JSON / FORMAT=TRADITIONAL -> mọi tool đọc EXPLAIN JSON hỏng
     - đổi plan có thể làm query đang nhanh chậm đi
   Script chỉ bật ở phạm vi SESSION và luôn tắt lại (kể cả khi query lỗi).

Cách dùng:
    MYSQL_PWD=xxx python3 scripts/hypergraph-bench.py --db cluvix_bench_tmp \\
        --file <file .sql chứa các câu SELECT cần đo> [--runs 3]
    (story-50 crm-standalone: file mẫu cũ `scripts/hypergraph-queries.sql` đã xoá — trỏ tới
    module `accountant`/`inventory` không còn tồn tại trong repo CRM standalone này; dựng lại
    file mẫu mới từ SQL thật trong handler đang cần đo khi cần dùng lại công cụ.)

An toàn — 3 cổng, có self-test bắt buộc chạy trước mọi query:
    1. Câu phải bắt đầu bằng SELECT/WITH.
    2. Không chứa từ khoá ghi/DDL ở bất kỳ đâu (INSERT/UPDATE/DROP/ALTER/...).
    3. Không chứa dạng SELECT-nhưng-nguy-hiểm: INTO OUTFILE/DUMPFILE (ghi ra đĩa), LOAD_FILE
       (đọc file máy chủ), SLEEP/BENCHMARK (treo/đốt CPU), GET_LOCK, FOR UPDATE.
    Vi phạm -> từ chối, exit 3. Self-test hỏng -> exit 2, không chạy gì.
    ⚠ Cổng 3 được thêm 2026-08-13 sau khi `/bmad-retro` phát hiện bản đầu CHO QUA
      `SELECT * FROM customer INTO OUTFILE '/tmp/leak.csv'` — tức ghi bệnh án ra đĩa, dù docblock
      lúc đó tuyên bố "chỉ đọc nên an toàn". Đừng nới cổng mà không thêm ca self-test tương ứng.
    - Cảnh báo (không chặn) nếu --db trùng DB_NAME trong backend/.env.

Cách đo (đọc kỹ trước khi tin số):
    - Thời gian lấy từ `actual time` của node gốc trong EXPLAIN ANALYZE — tức đo TRONG SERVER.
      KHÔNG dùng wall-clock của tiến trình: mỗi lần gọi `mysql` tốn ~25-30ms khởi động, trong khi
      query trên dữ liệu dev chạy <1ms, nên wall-clock chỉ đo chi phí spawn và cho chênh lệch
      ±100% ngẫu nhiên (đã dính đúng bẫy này lần đầu viết script).
    - Warm-up 1 lần rồi đo TRUNG VỊ của --runs lần.
    - Cả hai nhánh <1ms -> script tự báo "không kết luận được" thay vì in % vô nghĩa.
    - Đo trên DB dev volume nhỏ -> kết luận vẫn có thể NGƯỢC với production. Luôn xem số rows của
      bảng chính trước khi tin con số phần trăm.

Exit: 0 chạy xong · 2 self-test fail (cổng hỏng) · 3 lỗi vận hành / input không an toàn.
"""
import argparse
import os
import re
import statistics
import subprocess
import sys
import time

BE = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

# Cổng 1 — chỉ cho phép câu bắt đầu bằng SELECT hoặc WITH (CTE).
READONLY_START = re.compile(r'^\s*(SELECT|WITH)\b', re.I)

# Cổng 2 — từ khoá ghi/DDL ở bất kỳ đâu (kể cả subquery).
FORBIDDEN = re.compile(
    r'\b(INSERT|UPDATE|DELETE|REPLACE|ALTER|DROP|CREATE|TRUNCATE|GRANT|REVOKE|SET\s+GLOBAL|'
    r'LOAD\s+DATA|RENAME|CALL|LOCK\s+TABLES|FLUSH)\b', re.I)

# Cổng 3 — câu HÌNH DẠNG SELECT nhưng vẫn nguy hiểm. Hai cổng trên KHÔNG bắt được nhóm này:
# nó bắt đầu bằng SELECT và không chứa từ khoá ghi nào.
#   INTO OUTFILE/DUMPFILE -> GHI dữ liệu ra đĩa (ERP y tế: xuất bệnh án ra file tuỳ ý)
#   LOAD_FILE()           -> ĐỌC file tuỳ ý trên máy chủ DB
#   SLEEP/BENCHMARK       -> treo phiên / đốt CPU trên DB dùng chung
#   GET_LOCK              -> giữ lock, chặn phiên khác
#   FOR UPDATE / LOCK IN SHARE MODE -> khoá dòng, chặn ghi thật
# Phát hiện 2026-08-13 bởi `/bmad-retro` (F1) — QA story-05 chỉ thử đúng 1 input `DROP TABLE`,
# input đó lại bị Cổng 1 chặn, nên Cổng 2 chưa bao giờ thực sự được kiểm.
DANGEROUS_SELECT = re.compile(
    r'\bINTO\s+(OUTFILE|DUMPFILE)\b'
    r'|\b(SLEEP|BENCHMARK|GET_LOCK|RELEASE_LOCK|LOAD_FILE|SYS_EXEC|SYS_EVAL)\s*\('
    r'|\bFOR\s+UPDATE\b'
    r'|\bLOCK\s+IN\s+SHARE\s+MODE\b', re.I)

HYPERGRAPH_ON = "SET SESSION optimizer_switch='hypergraph_optimizer=on'"
HYPERGRAPH_OFF = "SET SESSION optimizer_switch='hypergraph_optimizer=off'"


def read_env_db_name():
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


def run_sql(args, sql, timeout=300):
    """Chạy 1 khối SQL, trả (stdout, elapsed_ms). Thoát 3 nếu lỗi vận hành."""
    cmd = ['mysql', '--batch', '--raw', '--skip-column-names',
           '-h', args.host, '-P', str(args.port), '-u', args.user, args.db, '-e', sql]
    env = dict(os.environ)
    if args.password is not None:
        env['MYSQL_PWD'] = args.password
    t0 = time.perf_counter()
    try:
        out = subprocess.run(cmd, capture_output=True, text=True, env=env, timeout=timeout)
    except FileNotFoundError:
        print('Không tìm thấy lệnh `mysql` trong PATH.', file=sys.stderr)
        sys.exit(3)
    except subprocess.TimeoutExpired:
        return None, None
    elapsed = (time.perf_counter() - t0) * 1000
    err = '\n'.join(l for l in out.stderr.splitlines() if 'Using a password' not in l).strip()
    if out.returncode != 0:
        return ('__ERROR__: ' + (err or out.stdout.strip())), elapsed
    return out.stdout, elapsed


def parse_queries(path):
    """Tách file thành list câu SQL (phân cách bằng `;` ở cuối dòng), bỏ comment `--`."""
    with open(path, encoding='utf-8') as fh:
        raw = fh.read()
    stripped = '\n'.join(l for l in raw.split('\n') if not l.strip().startswith('--'))
    return [q.strip() for q in stripped.split(';') if q.strip()]


def check_safe(queries):
    """3 cổng. Trả list (số thứ tự câu, lý do) — rỗng nghĩa là an toàn."""
    bad = []
    for i, q in enumerate(queries, 1):
        if not READONLY_START.match(q):
            bad.append((i, 'không bắt đầu bằng SELECT/WITH'))
        elif FORBIDDEN.search(q):
            bad.append((i, 'chứa từ khoá ghi/DDL: %s' % FORBIDDEN.search(q).group(0)))
        elif DANGEROUS_SELECT.search(q):
            bad.append((i, 'câu SELECT nhưng nguy hiểm: %s'
                        % ' '.join(DANGEROUS_SELECT.search(q).group(0).split())))
    return bad


# --- SELF-TEST ------------------------------------------------------------
# Quy ước bắt buộc của họ script quét trong thư mục này (idor-scan.py, body-id-scan.py,
# collation-scan.py): cổng an toàn phải có lưới đỏ tự động. Thiếu nó chính là lý do F1 sống sót.
# Ca ÂM TÍNH (phải bị chặn) quan trọng hơn ca dương tính — lưới chỉ có giá trị khi nó biết nói KHÔNG.
SELF_TEST_CASES = [
    # (nhãn, câu, có phải bị chặn không)
    ('SELECT thường',                     'SELECT id FROM customer', False),
    ('WITH (CTE)',                        'WITH x AS (SELECT 1 AS a) SELECT a FROM x', False),
    ('JOIN + UNION ALL',                  'SELECT a.id FROM appointment a JOIN customer c ON c.id=a.customer_id '
                                          'UNION ALL SELECT id FROM payment', False),
    ('cột tên created_at (không được nhầm là CREATE)',
                                          'SELECT created_at, dayCreate FROM appointment', False),
    ('INTO @var (script tự dùng, hợp lệ)', 'SELECT COUNT(*) INTO @n FROM customer', False),
    ('INTO OUTFILE',                      "SELECT * FROM customer INTO OUTFILE '/tmp/x.csv'", True),
    ('INTO DUMPFILE',                     "SELECT * FROM customer INTO DUMPFILE '/tmp/x.bin'", True),
    ('SLEEP()',                           'SELECT SLEEP(600)', True),
    ('BENCHMARK()',                       "SELECT BENCHMARK(100000000, SHA2('x',256))", True),
    ('GET_LOCK()',                        "SELECT GET_LOCK('x', 600)", True),
    ('LOAD_FILE()',                       "SELECT LOAD_FILE('/etc/passwd')", True),
    ('FOR UPDATE',                        'SELECT id FROM customer WHERE id=1 FOR UPDATE', True),
    ('LOCK IN SHARE MODE',                'SELECT id FROM customer LOCK IN SHARE MODE', True),
    ('DDL ở câu thứ 2',                   'DROP TABLE customer', True),
    ('UPDATE',                            'UPDATE customer SET name=1', True),
]


def self_test():
    failed = []
    for label, q, want_blocked in SELF_TEST_CASES:
        blocked = bool(check_safe([q]))
        if blocked != want_blocked:
            failed.append('  ✗ %s — mong đợi %s, nhận %s'
                          % (label, 'CHẶN' if want_blocked else 'cho qua',
                             'CHẶN' if blocked else 'cho qua'))
    if failed:
        print('SELF-TEST FAIL — cổng an toàn hỏng, KHÔNG chạy query nào:', file=sys.stderr)
        print('\n'.join(failed), file=sys.stderr)
        return False
    return True


# Node gốc của EXPLAIN ANALYZE FORMAT=TREE: "(actual time=<first>..<last> rows=... loops=...)"
ACTUAL_TIME_RE = re.compile(r'\(actual time=[\d.]+\.\.([\d.]+)\s')


def median_ms(args, sql, runs, hypergraph):
    """Đo THỜI GIAN THỰC THI TRONG SERVER qua EXPLAIN ANALYZE, không phải wall-clock của tiến trình.

    ⚠ Vì sao không dùng wall-clock: mỗi lần gọi `mysql` tốn ~25-30ms khởi động tiến trình, trong khi
    query report trên dữ liệu dev chỉ chạy <1ms -> wall-clock đo chi phí spawn chứ không đo query,
    cho ra chênh lệch ±100% hoàn toàn ngẫu nhiên. `actual time` của node gốc là số đo trong server.

    EXPLAIN ANALYZE (FORMAT=TREE mặc định) chạy được ở CẢ HAI nhánh — chỉ FORMAT=JSON/TRADITIONAL
    là hypergraph không hỗ trợ.
    """
    prefix = (HYPERGRAPH_ON if hypergraph else HYPERGRAPH_OFF) + '; '
    suffix = '; ' + HYPERGRAPH_OFF
    stmt = prefix + 'EXPLAIN ANALYZE ' + sql + suffix
    out, _ = run_sql(args, stmt)          # warm-up
    if out is None:
        return None, 'TIMEOUT'
    if out.startswith('__ERROR__'):
        return None, out[11:].strip()
    times = []
    for _ in range(runs):
        out, _ = run_sql(args, stmt)
        if out is None:
            return None, 'TIMEOUT'
        if out.startswith('__ERROR__'):
            return None, out[11:].strip()
        m = ACTUAL_TIME_RE.search(out)
        if not m:
            return None, 'không đọc được actual time từ EXPLAIN ANALYZE'
        times.append(float(m.group(1)))
    return statistics.median(times), None


def analyze_actual(args, sql):
    """Nhánh MẶC ĐỊNH: lấy `actual_last_row_ms`/`actual_rows` từ EXPLAIN ANALYZE FORMAT=JSON INTO @var.

    Bổ sung cho phần so sánh (đã dùng EXPLAIN ANALYZE FORMAT=TREE ở cả hai nhánh): dạng JSON cho số
    liệu có cấu trúc, dễ trích. CHỈ chạy được ở nhánh mặc định — hypergraph không hỗ trợ FORMAT=JSON.
    """
    out, _ = run_sql(args, HYPERGRAPH_OFF + '; EXPLAIN ANALYZE FORMAT=JSON INTO @plan ' + sql
                     + '; SELECT @plan')
    if out is None or out.startswith('__ERROR__'):
        return None, None
    m_time = re.search(r'"actual_last_row_ms"\s*:\s*"?([\d.]+)', out)
    m_rows = re.search(r'"actual_rows"\s*:\s*"?([\d.]+)', out)
    return (float(m_time.group(1)) if m_time else None,
            float(m_rows.group(1)) if m_rows else None)


def tree_plan(args, sql, hypergraph):
    prefix = (HYPERGRAPH_ON + '; ') if hypergraph else (HYPERGRAPH_OFF + '; ')
    out, _ = run_sql(args, prefix + 'EXPLAIN FORMAT=TREE ' + sql + '; ' + HYPERGRAPH_OFF)
    if out is None:
        return 'TIMEOUT'
    return out.strip()


def plan_summary(plan, width=68):
    """Dòng đầu của plan cây — chỉ để HIỂN THỊ, KHÔNG dùng để so sánh (xem plan_shape)."""
    if plan.startswith('__ERROR__'):
        return plan[11:].strip()[:width]
    first = next((l.strip() for l in plan.split('\n') if l.strip().startswith('->')), '')
    return first[:width]


# Bỏ phần số liệu để so CẤU TRÚC plan, không so chi phí ước lượng:
#   "(cost=1287..1544 rows=30.3)" và "(actual time=0.5..0.8 rows=569 loops=1)" thay đổi giữa các lần
#   chạy dù plan y hệt -> nếu không bỏ thì lần nào cũng báo "đổi plan".
PLAN_NOISE_RE = re.compile(r'\((?:cost|actual time)=[^)]*\)')


def plan_shape(plan):
    """Chuẩn hoá TOÀN BỘ cây plan để so sánh.

    Trước đây chỉ so dòng ĐẦU -> hai plan khác nhau ở tầng dưới mà trùng node gốc bị đếm nhầm là
    "giống nhau" (QA story-05 finding #13). Nay so cả cây, sau khi bỏ cost/actual time.
    """
    if plan.startswith('__ERROR__') or plan == 'TIMEOUT':
        return None
    lines = []
    for raw in plan.replace('\\n', '\n').split('\n'):
        s = PLAN_NOISE_RE.sub('', raw).rstrip()
        if s.strip():
            lines.append(s)
    return '\n'.join(lines)


def main():
    p = argparse.ArgumentParser(
        description='Đo plan/thời gian có và không có hypergraph optimizer (chỉ chẩn đoán thủ công).',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog='Exit: 0 xong · 2 self-test fail · 3 lỗi vận hành/input không an toàn')
    p.add_argument('--db', required=True, help='tên DB (ghi tường minh, ưu tiên *_tmp)')
    p.add_argument('--file', required=True, help='file .sql chứa các câu SELECT, phân cách bằng ;')
    p.add_argument('--runs', type=int, default=3, help='số lần đo sau warm-up (mặc định 3)')
    p.add_argument('--host', default='127.0.0.1')
    p.add_argument('--port', default=3306, type=int)
    p.add_argument('--user', default='root')
    p.add_argument('--password', default=os.environ.get('MYSQL_PWD'))
    args = p.parse_args()

    if not self_test():
        return 2

    if not os.path.isfile(args.file):
        print('Không thấy file: %s' % args.file, file=sys.stderr)
        return 3

    env_db = read_env_db_name()
    if env_db and args.db == env_db:
        print('⚠  `%s` là DB_NAME trong backend/.env. Script chỉ chạy SELECT nên an toàn,\n'
              '   nhưng ưu tiên đo trên bản clone *_tmp để không tranh IO với dev.\n' % args.db)

    queries = parse_queries(args.file)
    if not queries:
        print('File không có câu SQL nào.', file=sys.stderr)
        return 3

    bad = check_safe(queries)
    if bad:
        print('TỪ CHỐI — input không phải read-only:', file=sys.stderr)
        for i, why in bad:
            print('  câu #%d: %s' % (i, why), file=sys.stderr)
        return 3

    print('== hypergraph bench: %d câu, %d lần đo/câu (sau warm-up), DB `%s` ==\n'
          % (len(queries), args.runs, args.db))

    results = []
    try:
        for i, q in enumerate(queries, 1):
            label = ' '.join(q.split())[:60]
            base_ms, base_err = median_ms(args, q, args.runs, hypergraph=False)
            hyp_ms, hyp_err = median_ms(args, q, args.runs, hypergraph=True)
            results.append({
                'i': i, 'label': label,
                'base_ms': base_ms, 'base_err': base_err,
                'hyp_ms': hyp_ms, 'hyp_err': hyp_err,
                'base_raw': tree_plan(args, q, False),
                'hyp_raw': tree_plan(args, q, True),
            })
            print('#%d %s' % (i, label))
            if base_err or hyp_err:
                print('   LỖI  mặc định=%s | hypergraph=%s' % (base_err or 'ok', hyp_err or 'ok'))
            else:
                delta = (hyp_ms - base_ms) / base_ms * 100 if base_ms else 0
                # Dưới 1ms thì chênh lệch % vô nghĩa dù có lớn — nói thẳng thay vì kết luận bừa.
                if max(base_ms, hyp_ms) < 1.0:
                    verdict = 'CẢ HAI <1ms — dữ liệu quá nhỏ, không kết luận được'
                else:
                    verdict = ('hypergraph NHANH HƠN' if delta < -10 else
                               'hypergraph CHẬM HƠN' if delta > 10 else 'không khác đáng kể')
                print('   mặc định %8.3f ms | hypergraph %8.3f ms | %+6.1f%%  -> %s   '
                      '(actual time trong server, trung vị %d lần)'
                      % (base_ms, hyp_ms, delta, verdict, args.runs))
                a_ms, a_rows = analyze_actual(args, q)
                if a_ms is not None:
                    print('   EXPLAIN ANALYZE (mặc định): actual_last_row=%.1f ms, actual_rows=%s'
                          % (a_ms, int(a_rows) if a_rows is not None else '?'))
            r = results[-1]
            print('   plan mặc định : %s' % plan_summary(r['base_raw']))
            print('   plan hypergraph: %s' % plan_summary(r['hyp_raw']))
            a, b = plan_shape(r['base_raw']), plan_shape(r['hyp_raw'])
            r['plan_changed'] = None if (a is None or b is None) else (a != b)
            print('   -> plan %s\n' % {True: 'KHÁC NHAU (so cả cây)',
                                        False: 'GIỐNG NHAU (so cả cây)',
                                        None: 'không so được'}[r['plan_changed']])
    finally:
        # Bảo hiểm: phiên nào script mở cũng đã tắt hypergraph, nhưng gọi lại cho chắc.
        run_sql(args, HYPERGRAPH_OFF)

    ok = [r for r in results if not r['base_err'] and not r['hyp_err']]
    changed = [r for r in ok if r.get('plan_changed')]
    print('== Tổng kết ==')
    print('   %d/%d câu chạy được; %d câu ĐỔI PLAN khi bật hypergraph.' % (len(ok), len(results), len(changed)))
    if changed:
        print('   Câu đổi plan (đáng xem có thể vá bằng index/viết lại query):')
        for r in changed:
            print('     #%d  %+.1f%%  %s' % (r['i'], (r['hyp_ms'] - r['base_ms']) / r['base_ms'] * 100
                                             if r['base_ms'] else 0, r['label']))
    print('\n   Nhắc: KHÔNG bật hypergraph toàn cục. Nếu plan hypergraph tốt hơn -> sửa index/query để')
    print('   optimizer mặc định cũng chọn được plan đó.')
    return 0


if __name__ == '__main__':
    sys.exit(main())
