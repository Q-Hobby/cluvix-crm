#!/usr/bin/env bash
# verify-links.sh — kiểm link GIỮA CÁC FILE KNOWLEDGE có trỏ tới file có thật không.
#
# Vì sao cần (bài học 2026-08-17): `verify-paths.sh` chỉ kiểm đường dẫn CODE
# (backend/, internal/modules/, frontend/src/, mobile/lib/). Nó KHÔNG kiểm link
# knowledge→knowledge. Hệ quả thật: sau khi convert sang skill, 3 file router vẫn
# trỏ vào `skill/frontend/rules/` đã bị xoá, mà cả 4 gate đều báo xanh. Trạng thái
# nửa cũ nửa mới đó nguy hiểm hơn cũ hoàn toàn — agent đọc trúng nhánh chết sẽ kết
# luận "không có rule" rồi tự chế.
#
# Dùng:
#   ./verify-links.sh              # quét 3 repo
#   ./verify-links.sh --self-test
#
# Exit: 0 = sạch · 1 = có link chết · 2 = self-test trượt

set -uo pipefail
ROOT="${CLUVIX_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)}"

# Token là template do command sinh lúc chạy, KHÔNG phải link tĩnh → bỏ qua.
RUNTIME_TEMPLATES='^(brief|prd|architecture|epics|deferred-work|retrospective|test-log|final-report|baseline|path-audit|story-.*|.*-story)\.md$'

py() {
python3 - "$@" <<'PY'
import re, pathlib, sys, os
ROOT=pathlib.Path(os.environ['ROOT'])
RT=re.compile(os.environ['RUNTIME_TEMPLATES'])
roots=[ROOT/'skill', ROOT/'frontend/skill', ROOT/'mobile/skill']
pat=re.compile(r'`([A-Za-z0-9_./-]+\.md)`|\]\(([A-Za-z0-9_./-]+\.md)\)')
bad=[]
for root in roots:
    if not root.is_dir(): continue
    for f in root.rglob('*.md'):
        if '/.git/' in str(f): continue
        for i,line in enumerate(f.read_text().split('\n'),1):
            for m in pat.finditer(line):
                tok=m.group(1) or m.group(2)
                base=tok.rsplit('/',1)[-1]
                if tok.startswith(('http','bmad/')) or RT.match(base): continue
                # thử: cạnh file · reference/ cùng skill · gốc cây knowledge · gốc repo
                cands=[f.parent/tok, f.parent/'reference'/tok, root/tok, ROOT/tok, root.parent/tok]
                sd=f.parent
                while sd!=root and sd.parent!=sd:
                    cands.append(sd/tok); cands.append(sd/'reference'/tok); sd=sd.parent
                if any(c.exists() for c in cands): continue
                bad.append((str(f.relative_to(ROOT)), i, tok))
print(len(bad))
for b in bad: print(f"{b[0]}|{b[1]}|{b[2]}")
PY
}

self_test() {
  local t; t=$(mktemp -d); mkdir -p "$t/skill/skills/x/reference"
  printf 'link ok `reference/r.md` · link chết `khong_ton_tai.md`\n' >"$t/skill/skills/x/SKILL.md"
  printf 'noi dung\n' >"$t/skill/skills/x/reference/r.md"
  local out n
  out=$(ROOT="$t" RUNTIME_TEMPLATES="$RUNTIME_TEMPLATES" py); n=$(head -1 <<<"$out")
  rm -rf "$t"
  echo "── self-test ──"
  if [ "$n" = "1" ]; then echo "  ✔ link cạnh reference/ → OK · link bịa → bị bắt"; echo "  self-test PASS"; return 0; fi
  echo "  ✘ bắt được $n (cần đúng 1)"; echo "  self-test FAIL"; return 1
}

[ "${1:-}" = "--self-test" ] && { self_test; exit $?; }
self_test >/dev/null || { self_test; exit 2; }

OUT=$(ROOT="$ROOT" RUNTIME_TEMPLATES="$RUNTIME_TEMPLATES" py)
N=$(head -1 <<<"$OUT")
echo "verify-links: $N link knowledge chết"
if [ "$N" -gt 0 ]; then
  tail -n +2 <<<"$OUT" | awk -F'|' '{printf "  %s:%s  →  %s\n",$1,$2,$3}' | head -60
  [ "$N" -gt 60 ] && echo "  ... (còn $((N-60)))"
  echo
  echo "Link knowledge chết = agent đọc trúng nhánh đã gỡ, kết luận sai 'không có rule' rồi tự chế."
  exit 1
fi
echo "  ✅ mọi link giữa file knowledge đều trỏ tới file có thật"
