#!/usr/bin/env bash
# verify-paths.sh — kiểm mọi đường dẫn code nhắc trong knowledge base có tồn tại thật không.
#
# Vì sao cần: knowledge base trỏ tới file code; code dịch chuyển (đổi tên module, restructure)
# mà doc đứng yên thì doc TỰ TRỞ THÀNH SAI mà không file doc nào bị sửa — loại lỗi này không
# quét được bằng "xem file nào vừa đổi". Phải quét FULL, mỗi lần.
#
# Dùng:
#   ./verify-paths.sh                 # quét cả 3 repo
#   ./verify-paths.sh --repo skill    # skill | frontend | mobile
#   ./verify-paths.sh --self-test     # tự kiểm logic phân loại trên case đã biết
#   ./verify-paths.sh --audit         # in báo cáo đầy đủ 3 nhóm (OK / cần webapp/ / chết)
#
# Exit: 0 = không có path chết · 1 = có path chết · 2 = self-test trượt
#
# ⚠ KHÔNG quét bmad/ — plan/story là ghi chép lịch sử, path cũ trong đó là ĐÚNG với thời điểm
#   viết. Sửa chúng là làm sai lịch sử.

set -uo pipefail

ROOT="${CLUVIX_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)}"
BACKEND="$ROOT/backend"

# ── phân giải 1 token đường dẫn thành đường dẫn thật trên đĩa ────────────────
# in ra: "OK <path>" | "WEBAPP <path-đúng>" | "DEAD" | "SKIP <lý do>"
# $2 = gốc cây knowledge đang quét (để phân giải link nội bộ dạng backend/modules/x.md,
#      vốn nghĩa là skill/backend/modules/x.md chứ KHÔNG phải code Go ở backend/)
classify() {
  local tok="$1" kbase="${2:-}" base cand
  tok="${tok%/}"; tok="${tok%.}"

  # link nội bộ trong knowledge → phân giải theo gốc cây knowledge trước.
  # Kể cả token không có đuôi (vd `backend/prompts/patterns`) và token là thư mục.
  # FE/mobile còn dùng quy ước "path bắt đầu backend/ = knowledge repo CHA"
  # (xem frontend/CLAUDE.md §Quan hệ với repo cha) → thử thêm $ROOT/skill.
  local kb
  for kb in "$kbase" "$ROOT/skill"; do
    [ -n "$kb" ] || continue
    for ext in "" .md; do
      [ -e "$kb/$tok$ext" ] && { echo "OK $tok"; return; }
    done
  done

  # allowlist: doc nhắc đúng về thứ cố ý không tồn tại (xem verify-paths-allow.txt)
  local ALLOW="$(dirname "${BASH_SOURCE[0]}")/verify-paths-allow.txt"
  if [ -f "$ALLOW" ] && grep -qE "^${tok%/} \|" "$ALLOW" 2>/dev/null; then echo "SKIP allowlisted"; return; fi

  # loại token bị regex cắt cụt: kết thúc bằng _ hoặc quá ngắn
  case "$tok" in
    *_) echo "SKIP prefix-cut"; return ;;
    *xxx*|*'<'*|*'{'*) echo "SKIP placeholder"; return ;;   # backend/scripts/gen_xxx_template/
    backend/tmp/*)     echo "SKIP runtime-artifact"; return ;;  # run.log, run_dev.lock
  esac
  local depth; depth=$(awk -F/ '{print NF}' <<<"$tok")
  if [ "$depth" -lt 3 ]; then echo "SKIP too-shallow"; return; fi

  case "$tok" in
    internal/modules/*) base="$BACKEND" ;;
    backend/*|frontend/*|mobile/*) base="$ROOT" ;;
    *) echo "SKIP unknown-prefix"; return ;;
  esac

  for ext in "" .go .ts .dart .json .sql .md .html .scss; do
    cand="$base/$tok$ext"
    [ -e "$cand" ] && { echo "OK $tok"; return; }
  done

  # thử thêm prefix webapp/ (di chứng restructure module 2026-07-28)
  case "$tok" in
    internal/modules/webapp/*) : ;;   # đã có rồi, không thêm nữa (chống webapp/webapp/)
    internal/modules/*)
      local w="internal/modules/webapp/${tok#internal/modules/}"
      for ext in "" .go .json .sql; do
        [ -e "$BACKEND/$w$ext" ] && { echo "WEBAPP $w"; return; }
      done ;;
  esac

  # Doc hay viết `<đường-dẫn>/<TênHàm>` (vd handler/ImportCustomerFromExcel).
  # Thư mục cha có thật + đoạn cuối viết hoa đầu = tham chiếu hàm/type Go, không phải file.
  local parent="${tok%/*}" leaf="${tok##*/}"
  case "$leaf" in
    [A-Z]*) [ -d "$base/$parent" ] && { echo "SKIP func-ref"; return; } ;;
  esac

  echo "DEAD"
}

# ── self-test: logic phân loại phải đúng trên case đã biết trước ─────────────
# Không có bước này thì script xanh không chứng minh được gì — nó có thể xanh vì
# không tìm thấy token nào (vd glob không quote bị shell nuốt).
self_test() {
  local fail=0
  check_kb() { local got; got=$(classify "$1" "$ROOT/skill" | awk "{print \$1}"); if [ "$got" = "$2" ]; then echo "  ✔ [kb] $1 → $got"; else echo "  ✘ [kb] $1 → $got (kỳ vọng $2)"; fail=1; fi; }
  check() { # <token> <kỳ vọng>
    local got; got=$(classify "$1" | awk '{print $1}')
    if [ "$got" = "$2" ]; then echo "  ✔ $1 → $got"
    else echo "  ✘ $1 → $got (kỳ vọng $2)"; fail=1; fi
  }
  echo "── self-test ──"
  check "internal/modules/webapp/crm/customer/handler/customer" OK
  check "internal/modules/crm/customer/handler/customer"        WEBAPP
  check "internal/modules/khong_he_ton_tai/handler/x"           DEAD
  check "internal/modules/config/emr_"                          SKIP
  check "backend/pkg/tenant/scope"                              OK
  check_kb "skills/backend-hard-rules/SKILL.md" OK
  [ "$fail" = 0 ] && { echo "  self-test PASS"; return 0; }
  echo "  self-test FAIL — KHÔNG tin kết quả quét bên dưới"; return 1
}

# ── trích token từ 1 cây knowledge ──────────────────────────────────────────
# ⚠ --include PHẢI quote: viết trần *.md thì zsh nuốt glob → 0 hit → lưới luôn xanh.
extract() {
  local dir="$1"
  grep -rhoE '(internal/modules|backend|frontend/src|mobile/lib)/[A-Za-z0-9_./-]+' \
       "$dir" --include='*.md' 2>/dev/null | sort -u
}

scan() {
  local name="$1" dir="$2"
  [ -d "$dir" ] || return 0
  local tok verdict path
  while IFS= read -r tok; do
    [ -z "$tok" ] && continue
    read -r verdict path <<<"$(classify "$tok" "$dir")"
    case "$verdict" in
      OK)     N_OK=$((N_OK+1)) ;;
      WEBAPP) N_WEBAPP=$((N_WEBAPP+1)); echo "$name|$tok|$path" >>"$TMP/webapp" ;;
      DEAD)   N_DEAD=$((N_DEAD+1));     echo "$name|$tok" >>"$TMP/dead" ;;
      SKIP)   N_SKIP=$((N_SKIP+1));     echo "$name|$tok|$path" >>"$TMP/skip" ;;
    esac
  done < <(extract "$dir")
}

# ── main ────────────────────────────────────────────────────────────────────
MODE=scan; ONLY=""
while [ $# -gt 0 ]; do
  case "$1" in
    --self-test) MODE=selftest ;;
    --audit)     MODE=audit ;;
    --repo)      ONLY="$2"; shift ;;
    *) echo "tham số lạ: $1"; exit 2 ;;
  esac; shift
done

if [ "$MODE" = selftest ]; then self_test; exit $?; fi
self_test >/dev/null || { self_test; exit 2; }

TMP=$(mktemp -d); trap 'rm -rf "$TMP"' EXIT
: >"$TMP/webapp"; : >"$TMP/dead"; : >"$TMP/skip"
N_OK=0; N_WEBAPP=0; N_DEAD=0; N_SKIP=0

[ -z "$ONLY" ] || [ "$ONLY" = skill ]    && scan skill    "$ROOT/skill"
[ -z "$ONLY" ] || [ "$ONLY" = frontend ] && scan frontend "$ROOT/frontend/skill"
[ -z "$ONLY" ] || [ "$ONLY" = mobile ]   && scan mobile   "$ROOT/mobile/skill"

TOTAL=$((N_OK+N_WEBAPP+N_DEAD+N_SKIP))
echo "verify-paths: $TOTAL token | OK=$N_OK · cần webapp/=$N_WEBAPP · CHẾT=$N_DEAD · bỏ qua=$N_SKIP"

if [ "$MODE" = audit ]; then
  echo; echo "── cần thêm prefix webapp/ ($N_WEBAPP) ──"; sed 's/|/  →  /g' "$TMP/webapp"
  echo; echo "── bỏ qua (token cụt/không phân giải được) ($N_SKIP) ──"; sed 's/|/  /g' "$TMP/skip"
fi

if [ "$N_DEAD" -gt 0 ]; then
  echo; echo "── ĐƯỜNG DẪN CHẾT ($N_DEAD) ──"; sed 's/|/  /g' "$TMP/dead"
  echo; echo "Sửa hoặc gỡ các mục trên. Không có file thật = agent sẽ kết luận sai 'module không tồn tại'."
  exit 1
fi
[ "$N_WEBAPP" -gt 0 ] && { echo "⚠ còn $N_WEBAPP token cần prefix webapp/ (chạy --audit để xem)"; exit 1; }
echo "✅ mọi đường dẫn code trong knowledge base đều tồn tại thật."
