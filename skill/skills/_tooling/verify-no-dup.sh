#!/usr/bin/env bash
# verify-no-dup.sh — bắt file trùng nội dung mà KHÔNG phải symlink.
#
# Vì sao: knowledge base này phục vụ 3 repo + nhiều agent. Cách duy nhất giữ 1 nguồn
# sự thật là symlink. Bản sao thật sẽ DRIFT — đã xảy ra: 6 persona × 3 repo = 18 file,
# cả 18 khác hash nhau, không xác định được bản nào chuẩn.
#
# Quét: skill/ của 3 repo + .claude/commands + .claude/skills + bmad/personas.
# Bỏ qua file quá ngắn (<15 dòng) — trùng nhau thường là ngẫu nhiên, không phải nhân bản.
#
# ⚠ CHỈ so trong CÙNG MỘT REPO (bài học story-19). `frontend/` và `mobile/` là repo
# git độc lập, phải chạy được standalone — symlink từ `frontend/.claude/commands/`
# sang `skill/commands/` của repo cha sẽ GÃY ngay khi clone riêng frontend. Nên hai
# repo cùng giữ một bản command/persona là ĐÚNG, không phải nhân bản cần dọn.
# Luật "1 nguồn sự thật, bản sao là symlink" chỉ khả thi trong phạm vi 1 repo.
#
# Exit: 0 = sạch · 1 = có bản sao · 2 = self-test trượt

set -uo pipefail
ROOT="${CLUVIX_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)}"
MIN_LINES=15

scan_dirs() {
  for d in \
    "$ROOT/skill" "$ROOT/frontend/skill" "$ROOT/mobile/skill" \
    "$ROOT/.claude/commands" "$ROOT/frontend/.claude/commands" "$ROOT/mobile/.claude/commands" \
    "$ROOT/bmad/personas" "$ROOT/frontend/bmad/personas" "$ROOT/mobile/bmad/personas"
  do [ -d "$d" ] && echo "$d"; done
}

collect() { # in: <hash> <path>  — bỏ symlink (symlink CHÍNH LÀ cách làm đúng)
  local d
  while IFS= read -r d; do
    find "$d" -name '*.md' -type f ! -path '*/.git/*' 2>/dev/null
  done < <(scan_dirs) | sort -u | while IFS= read -r f; do
    [ -L "$f" ] && continue
    local l; l=$(wc -l <"$f" | tr -d ' ')
    [ "$l" -lt "$MIN_LINES" ] && continue
    r="root"; case "$f" in "$ROOT"/frontend/*) r=frontend;; "$ROOT"/mobile/*) r=mobile;; esac
    echo "$r:$(md5 -q "$f" 2>/dev/null || md5sum "$f" | cut -d' ' -f1) $f"
  done
}

self_test() {
  local t; t=$(mktemp -d)
  seq 1 20 >"$t/a.md"; cp "$t/a.md" "$t/b.md"; ln -s "$t/a.md" "$t/c.md"
  local dup; dup=$(for f in "$t"/*.md; do [ -L "$f" ] && continue; echo "$(md5 -q "$f") $f"; done \
                   | awk '{print $1}' | sort | uniq -d | wc -l | tr -d ' ')
  local nsym; nsym=$(find "$t" -name '*.md' -type l | wc -l | tr -d ' ')
  rm -rf "$t"
  echo "── self-test ──"
  if [ "$dup" = 1 ] && [ "$nsym" = 1 ]; then
    echo "  ✔ bản sao thật → bị bắt · symlink → bỏ qua"; echo "  self-test PASS"; return 0
  fi
  echo "  ✘ dup=$dup (cần 1) · symlink=$nsym (cần 1)"; echo "  self-test FAIL"; return 1
}

[ "${1:-}" = "--self-test" ] && { self_test; exit $?; }
self_test >/dev/null || { self_test; exit 2; }

TMP=$(mktemp); trap 'rm -f "$TMP"' EXIT
collect >"$TMP"
TOTAL=$(wc -l <"$TMP" | tr -d ' ')
DUPH=$(awk '{print $1}' "$TMP" | sort | uniq -d)

echo "verify-no-dup:"
if [ -z "$DUPH" ]; then
  echo "  $TOTAL file (không tính symlink) · 0 bản sao"
  echo "  ✅ không có file nhân bản"
  exit 0
fi
n=0
while IFS= read -r h; do
  [ -z "$h" ] && continue; n=$((n+1))
  echo "  ✘ trùng nội dung (nên là symlink về 1 nguồn):"
  awk -v h="$h" '$1==h{print "      " $2}' "$TMP" | sed "s|$ROOT/||"
done <<<"$DUPH"
echo "  $TOTAL file · $n nhóm bản sao"
exit 1
