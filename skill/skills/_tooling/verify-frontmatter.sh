#!/usr/bin/env bash
# verify-frontmatter.sh — kiểm frontmatter của mọi SKILL.md theo chuẩn Agent Skills.
#
# Luật kiểm:
#   1. Có frontmatter YAML mở/đóng bằng ---
#   2. `name` khớp CHÍNH XÁC tên thư mục, kebab-case, ≤64 ký tự
#   3. `description` không rỗng, ≤1024 ký tự
#   4. KHÔNG có field nào ngoài name/description  ← ràng buộc TRUNG LẬP AGENT (FR29)
#      skill/ phục vụ nhiều agent (Claude/Codex/Gemini). Field riêng của một runtime
#      (allowed-tools...) làm knowledge dính chặt vào runtime đó.
#   5. `description` nên nêu CẢ "làm gì" lẫn "KHI NÀO dùng" — thiếu vế "khi nào" là
#      nguyên nhân số 1 khiến skill không được kích hoạt. (cảnh báo, không chặn)
#
# Exit: 0 = sạch · 1 = có lỗi · 2 = self-test trượt

set -uo pipefail
ROOT="${CLUVIX_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)}"

ERR=0; WARN=0; N=0
err()  { echo "  ✘ $1"; ERR=$((ERR+1)); }
warn() { echo "  ⚠ $1"; WARN=$((WARN+1)); }

check_skill() {
  local f="$1" dir name desc fields
  dir=$(basename "$(dirname "$f")")
  N=$((N+1))

  head -1 "$f" | grep -q '^---$' || { err "$dir: không mở bằng ---"; return; }

  # lấy khối frontmatter (giữa 2 dòng ---)
  local fm; fm=$(awk 'NR>1 && /^---$/{exit} NR>1' "$f")

  # field ở cấp gốc (không thụt đầu dòng)
  fields=$(grep -E '^[a-zA-Z_-]+:' <<<"$fm" | sed 's/:.*//')
  local bad
  bad=$(grep -vE '^(name|description)$' <<<"$fields" || true)
  [ -n "$bad" ] && err "$dir: field lạ trong frontmatter → $(tr '\n' ' ' <<<"$bad") (chỉ được name/description — xem FR29 trung lập agent)"

  name=$(sed -n 's/^name:[[:space:]]*//p' <<<"$fm" | head -1 | tr -d '"'"'"'')
  [ -z "$name" ] && { err "$dir: thiếu name"; return; }
  [ "$name" = "$dir" ] || err "$dir: name='$name' KHÁC tên thư mục"
  [ ${#name} -le 64 ] || err "$dir: name dài ${#name} ký tự (>64)"
  grep -qE '^[a-z0-9]+(-[a-z0-9]+)*$' <<<"$name" || err "$dir: name không phải kebab-case"

  desc=$(sed -n '/^description:/,$p' <<<"$fm" | sed '1s/^description:[[:space:]]*//' | tr '\n' ' ')
  desc=$(sed -E 's/^[>|][-+]?[[:space:]]*//; s/^[[:space:]]+//; s/[[:space:]]+$//' <<<"$desc")
  [ -z "$desc" ] && { err "$dir: description rỗng"; return; }
  [ ${#desc} -le 1024 ] || err "$dir: description dài ${#desc} ký tự (>1024)"

  # vế "khi nào": tìm từ khoá chỉ tình huống dùng
  grep -qiE 'khi |trước khi|bất cứ khi|dùng skill này|đụng ' <<<"$desc" \
    || warn "$dir: description có thể thiếu vế \"KHI NÀO dùng\" — skill sẽ khó được kích hoạt"
}

self_test() {
  local t; t=$(mktemp -d); mkdir -p "$t/tot-skill" "$t/sai-skill"
  printf -- '---\nname: tot-skill\ndescription: Làm X. Dùng khi đụng Y.\n---\nnội dung\n' >"$t/tot-skill/SKILL.md"
  printf -- '---\nname: khong-khop\ndescription: \nallowed-tools: Bash\n---\n' >"$t/sai-skill/SKILL.md"
  local se=0 sw=0 keep_err=$ERR keep_warn=$WARN keep_n=$N
  ERR=0; WARN=0; N=0
  echo "── self-test ──"
  check_skill "$t/tot-skill/SKILL.md" >/dev/null; se=$ERR
  ERR=0; check_skill "$t/sai-skill/SKILL.md" >/dev/null; sw=$ERR
  rm -rf "$t"
  ERR=$keep_err; WARN=$keep_warn; N=$keep_n
  if [ "$se" = 0 ] && [ "$sw" -ge 2 ]; then echo "  ✔ file đúng → 0 lỗi · file sai → $sw lỗi"; echo "  self-test PASS"; return 0; fi
  echo "  ✘ file đúng cho $se lỗi (cần 0), file sai cho $sw lỗi (cần ≥2)"; echo "  self-test FAIL"; return 1
}

[ "${1:-}" = "--self-test" ] && { self_test; exit $?; }
self_test >/dev/null || { self_test; exit 2; }

echo "verify-frontmatter:"
while IFS= read -r f; do check_skill "$f"; done < <(
  find "$ROOT/skill/skills" "$ROOT/frontend/skill/skills" "$ROOT/mobile/skill/skills" \
       -name 'SKILL.md' 2>/dev/null | sort
)
echo "  $N skill · $ERR lỗi · $WARN cảnh báo"
[ "$ERR" -gt 0 ] && exit 1
echo "  ✅ frontmatter hợp lệ"
