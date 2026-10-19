#!/usr/bin/env bash
# verify-depth.sh — kiểm cấu trúc progressive disclosure của skill.
#
# Luật kiểm:
#   1. Body SKILL.md < 500 dòng (không tính frontmatter) — ngưỡng chuẩn Agent Skills.
#      Ngưỡng 350 dòng CŨ của HOW_TO_UPDATE.md đã bị thay, không dùng song song.
#   2. Reference chỉ sâu ĐÚNG 1 tầng: SKILL.md -> reference/x.md.
#      CẤM reference/x.md -> reference/y.md (agent phải lần nhiều tầng mới tới nội dung).
#   3. File reference > 100 dòng phải có mục lục (heading "Mục lục"/"Nội dung" hoặc
#      danh sách link nội bộ ở 25 dòng đầu).
#   4. Không có thư mục con trong reference/ (làm sâu quá 1 tầng).
#
# Exit: 0 = sạch · 1 = có lỗi · 2 = self-test trượt

set -uo pipefail
ROOT="${CLUVIX_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)}"
MAX_BODY=500
MAX_NO_TOC=100

ERR=0; N=0; NR=0
err() { echo "  ✘ $1"; ERR=$((ERR+1)); }

body_lines() { # bỏ khối frontmatter đầu file
  awk 'NR==1 && /^---$/{fm=1; next} fm==1 && /^---$/{fm=2; next} fm!=1' "$1" | wc -l | tr -d ' '
}

has_toc() {
  head -25 "$1" | grep -qiE '^#+ *(mục lục|nội dung|toc)|^- *\[.+\]\(#' && return 0
  return 1
}

check_skill_dir() {
  local d="$1" name; name=$(basename "$d")
  local sk="$d/SKILL.md"
  [ -f "$sk" ] || { err "$name: thiếu SKILL.md"; return; }
  N=$((N+1))

  local bl; bl=$(body_lines "$sk")
  [ "$bl" -lt "$MAX_BODY" ] || err "$name: body $bl dòng (≥$MAX_BODY) — đẩy bớt xuống reference/"

  [ -d "$d/reference" ] || return 0

  # INDEX.md bên trong cây skills/ = tàn dư chuỗi INDEX lồng đã gỡ (retro F4).
  # Bất biến thật là "không có INDEX.md trong skills/", KHÔNG phải "toàn repo chỉ có 3 INDEX.md"
  # — skill/hooks/ và skill/commands/ index tài sản KHÔNG phải knowledge, được giữ có chủ đích.
  local idx; idx=$(find "$d" -name 'INDEX.md' 2>/dev/null | head -1)
  [ -n "$idx" ] && err "$name: có INDEX.md trong cây skill ($(basename "$(dirname "$idx")")/INDEX.md) — chuỗi INDEX lồng đã được thay bằng router + description"

  # thư mục con trong reference/ = sâu quá 1 tầng
  local sub; sub=$(find "$d/reference" -mindepth 1 -type d 2>/dev/null)
  [ -n "$sub" ] && err "$name: reference/ có thư mục con ($(basename "$sub")) — chỉ được phẳng 1 tầng"

  local r rn
  while IFS= read -r r; do
    [ -z "$r" ] && continue
    NR=$((NR+1)); rn=$(basename "$r")
    # reference trỏ sang reference khác = 2 tầng
    grep -qE '\]\(\.?/?reference/|`reference/' "$r" \
      && err "$name/$rn: trỏ sang reference/ khác — reference phải là tầng CUỐI"
    local l; l=$(wc -l <"$r" | tr -d ' ')
    if [ "$l" -gt "$MAX_NO_TOC" ] && ! has_toc "$r"; then
      err "$name/$rn: $l dòng (>$MAX_NO_TOC) mà không có mục lục ở đầu"
    fi
  done < <(find "$d/reference" -maxdepth 1 -name '*.md' 2>/dev/null | sort)
}

self_test() {
  local t; t=$(mktemp -d)
  mkdir -p "$t/tot/reference" "$t/sai/reference/long-nhau"
  printf -- '---\nname: tot\ndescription: x\n---\nngắn gọn\n' >"$t/tot/SKILL.md"
  printf -- '# Mục lục\n- [a](#a)\n' >"$t/tot/reference/a.md"
  printf -- '---\nname: sai\ndescription: x\n---\n' >"$t/sai/SKILL.md"
  seq 1 600 >>"$t/sai/SKILL.md"
  printf 'xem `reference/khac.md`\n' >"$t/sai/reference/b.md"
  seq 1 150 >>"$t/sai/reference/b.md"
  local keep=$ERR ke=$N kr=$NR; ERR=0; N=0; NR=0
  echo "── self-test ──"
  check_skill_dir "$t/tot" >/dev/null; local a=$ERR
  ERR=0; check_skill_dir "$t/sai" >/dev/null; local b=$ERR
  rm -rf "$t"; ERR=$keep; N=$ke; NR=$kr
  if [ "$a" = 0 ] && [ "$b" -ge 3 ]; then
    echo "  ✔ skill đúng → 0 lỗi · skill sai → $b lỗi (body dài + reference lồng + thiếu mục lục)"
    echo "  self-test PASS"; return 0
  fi
  echo "  ✘ đúng=$a lỗi (cần 0) · sai=$b lỗi (cần ≥3)"; echo "  self-test FAIL"; return 1
}

[ "${1:-}" = "--self-test" ] && { self_test; exit $?; }
self_test >/dev/null || { self_test; exit 2; }

echo "verify-depth:"
while IFS= read -r d; do check_skill_dir "$d"; done < <(
  find "$ROOT/skill/skills" "$ROOT/frontend/skill/skills" "$ROOT/mobile/skill/skills" \
       -mindepth 1 -maxdepth 1 -type d ! -name '_*' 2>/dev/null | sort
)
echo "  $N skill · $NR file reference · $ERR lỗi"
[ "$ERR" -gt 0 ] && exit 1
echo "  ✅ cấu trúc progressive disclosure hợp lệ"
