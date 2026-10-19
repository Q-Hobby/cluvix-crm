#!/usr/bin/env bash
# verify-orphan.sh — kiểm file knowledge có ĐƯỜNG ĐI TỚI từ một cửa vào không.
#
# Vì sao cần (bài học 2026-09-02): `verify-links.sh` chỉ kiểm link ĐI RA có chết không.
# Nó KHÔNG kiểm chiều ngược lại — file có ai trỏ tới không. Hệ quả thật: `skill/commands/INDEX.md`
# (bảng 16 slash command, viết rất kỹ) không được file nào trỏ tới suốt nhiều tháng, cả 6 gate
# đều xanh, và không agent nào từng đọc nó. Kiến thức viết ra mà không ai tới được thì tốn công
# y hệt như không viết, nhưng nguy hiểm hơn: người viết tin là đã phổ biến rồi.
#
# Không chỉ đếm inbound link mà xét ĐƯỜNG ĐI (BFS từ cửa vào): file chỉ được một file mồ côi
# khác trỏ tới thì vẫn không ai tới được — cả cụm mồ côi cùng lúc là ca hay gặp sau refactor.
#
# Cửa vào (agent tới được KHÔNG cần link):
#   - <tree>/INDEX.md           router, luôn được nạp (skill/INDEX.md = CLAUDE.md)
#   - <tree>/skills/*/SKILL.md  harness tự kích hoạt theo `description`
#   - skill/commands/*.md       slash command, user gõ thẳng
# Ngoại lệ cố ý → thêm dòng vào verify-orphan-allow.txt (path tương đối gốc repo).
#
# Dùng:
#   ./verify-orphan.sh
#   ./verify-orphan.sh --self-test
#
# Exit: 0 = sạch · 1 = có file mồ côi · 2 = self-test trượt

set -uo pipefail
ROOT="${CLUVIX_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)}"
ALLOW="${CLUVIX_ALLOW:-$(dirname "${BASH_SOURCE[0]}")/verify-orphan-allow.txt}"

py() {
python3 - <<'PY'
import re, pathlib, os, collections
# .resolve(): trên macOS /var là symlink tới /private/var, còn danh sách file đã resolve()
# — không chuẩn hoá ROOT thì relative_to() ném ValueError và self-test tưởng là 0 hit.
ROOT = pathlib.Path(os.environ['ROOT']).resolve()
allow = set()
ap = pathlib.Path(os.environ.get('ALLOW', ''))
if ap.is_file():
    for line in ap.read_text().split('\n'):
        line = line.split('#', 1)[0].strip()
        if line:
            allow.add(line)   # kết thúc bằng "/" = cả thư mục (khớp theo tiền tố)

trees = [t for t in (ROOT/'skill', ROOT/'frontend/skill', ROOT/'mobile/skill') if t.is_dir()]

files = []
for t in trees:
    for f in t.rglob('*.md'):
        sf = str(f)
        if '/.git/' in sf or '/.claude/' in sf:
            continue   # .git = nội bộ git · .claude = artifact phiên (RESUME.md…), không phải knowledge
        files.append(f.resolve())
files = sorted(set(files))

# Pattern CỐ Ý lỏng hơn verify-links.sh (không bắt buộc backtick / cú pháp link markdown):
# hai gate sai về hai hướng ngược nhau. verify-links buộc chặt vì bắt nhầm prose thành link sẽ
# tố cáo "link chết" oan. Gate này thì bắt lỏng mới an toàn: mọi cách nhắc tên file đều là một
# đường agent có thể lần theo, bỏ sót cạnh = vu cho file lành là mồ côi. Ca thật: mô tả skill
# working-on-hr viết "chấm công → reference/hr-attendance.md" KHÔNG backtick — pattern chặt
# báo oan 4 file hr.
pat = re.compile(r'([A-Za-z0-9_][A-Za-z0-9_./-]*\.md)')

def resolve(f, tok, tree):
    cands = [f.parent/tok, f.parent/'reference'/tok, tree/tok, ROOT/tok, tree.parent/tok]
    sd = f.parent
    while sd != tree and sd.parent != sd:
        cands.append(sd/tok); cands.append(sd/'reference'/tok); sd = sd.parent
    for c in cands:
        try:
            if c.is_file():
                return c.resolve()
        except OSError:
            pass
    return None

edges = collections.defaultdict(set)
for f in files:
    tree = next((t for t in trees if str(f).startswith(str(t.resolve()))), None)
    if tree is None:
        continue
    try:
        text = f.read_text()
    except OSError:
        continue
    for m in pat.finditer(text):
        tok = m.group(1)
        if tok.startswith('http'):
            continue
        tgt = resolve(f, tok, tree.resolve())
        if tgt is not None:
            edges[f].add(tgt)

entries = set()
for t in trees:
    tr = t.resolve()
    idx = tr/'INDEX.md'
    if idx.is_file():
        entries.add(idx.resolve())
    for s in tr.glob('skills/*/SKILL.md'):
        entries.add(s.resolve())
    for c in tr.glob('commands/*.md'):
        if c.name != 'INDEX.md':
            entries.add(c.resolve())

seen = set(entries)
q = collections.deque(entries)
while q:
    cur = q.popleft()
    for nxt in edges.get(cur, ()):
        if nxt not in seen:
            seen.add(nxt); q.append(nxt)

orphans = []
for f in files:
    if f in seen:
        continue
    rel = str(f.relative_to(ROOT))
    if rel in allow or any(a.endswith('/') and rel.startswith(a) for a in allow):
        continue
    orphans.append(rel)

print(len(orphans))
for o in sorted(orphans):
    print(o)
PY
}

self_test() {
  local t; t=$(mktemp -d); mkdir -p "$t/skill/skills/x"
  # INDEX → a.md → b.md (b tới được GIÁN TIẾP) · c.md không ai trỏ · d.md chỉ được c (mồ côi) trỏ
  printf 'router `a.md`\n'          >"$t/skill/INDEX.md"
  printf 'đi tiếp `b.md`\n'         >"$t/skill/a.md"
  printf 'lá\n'                     >"$t/skill/b.md"
  printf 'mồ côi, trỏ `d.md`\n'     >"$t/skill/c.md"
  printf 'chỉ được c trỏ\n'         >"$t/skill/d.md"
  printf 'skill đầu vào\n'          >"$t/skill/skills/x/SKILL.md"
  local out n
  out=$(ROOT="$t" ALLOW=/dev/null py); n=$(head -1 <<<"$out")
  local got; got=$(tail -n +2 <<<"$out" | tr '\n' ' ')
  rm -rf "$t"
  echo "── self-test ──"
  if [ "$n" = "2" ] && [[ "$got" == *"skill/c.md"* && "$got" == *"skill/d.md"* ]]; then
    echo "  ✔ b.md tới được gián tiếp qua a.md → KHÔNG báo"
    echo "  ✔ c.md không ai trỏ → bị bắt · d.md chỉ được file mồ côi trỏ → cũng bị bắt"
    echo "  self-test PASS"; return 0
  fi
  echo "  ✘ bắt được $n ($got) — cần đúng 2: skill/c.md skill/d.md"
  echo "  self-test FAIL"; return 1
}

[ "${1:-}" = "--self-test" ] && { self_test; exit $?; }
self_test >/dev/null || { self_test; exit 2; }

OUT=$(ROOT="$ROOT" ALLOW="$ALLOW" py)
N=$(head -1 <<<"$OUT")
echo "verify-orphan: $N file knowledge không có đường đi tới"
if [ "$N" -gt 0 ]; then
  tail -n +2 <<<"$OUT" | sed 's/^/  /' | head -60
  [ "$N" -gt 60 ] && echo "  ... (còn $((N-60)))"
  echo
  echo "Không tới được = không agent nào đọc. Sửa bằng 1 trong 2:"
  echo "  · thêm link từ một file ĐÃ tới được (thường là INDEX.md của cây đó)"
  echo "  · cố ý đứng riêng → thêm path vào skill/skills/_tooling/verify-orphan-allow.txt kèm lý do"
  exit 1
fi
echo "  ✅ mọi file knowledge đều có đường đi từ một cửa vào"
