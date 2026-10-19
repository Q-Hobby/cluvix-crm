#!/usr/bin/env bash
# optimize.sh — chạy lại việc tối ưu knowledge base, chỉ soi phần MỚI kể từ lần trước.
#
# ⚠ Đây là bash thuần, CỐ Ý không phải slash command. Knowledge base này phục vụ nhiều
# agent (Claude, Codex, Gemini) và cả người — đặt logic trong slash command là khoá năng
# lực bảo trì vào một runtime, đúng cái mà cả epic skill-optimization đi tránh.
# `/skill-optimize` chỉ là vỏ mỏng gọi script này.
#
# Dùng:
#   ./optimize.sh            # tăng dần: FULL script + delta kể từ mốc lần trước
#   ./optimize.sh --full     # bỏ qua mốc, coi như lần đầu
#   ./optimize.sh --stamp    # chốt mốc mới (CHỈ chạy khi mọi thứ đã sạch)
#
# Exit: 0 = không còn việc · 1 = có việc phải làm · 2 = lỗi cấu hình

set -uo pipefail
TOOLING="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="${CLUVIX_ROOT:-$(cd "$TOOLING/../../.." && pwd)}"
STATE="$TOOLING/../maintaining-skill-base/state.json"

# ── Hai tầng, KHÔNG gộp được ────────────────────────────────────────────────
# FULL (rẻ, script): bắt lỗi do MÔI TRƯỜNG trôi — code đổi chỗ, doc đứng yên và TỰ trở
#   thành sai mà không file doc nào bị sửa. Quét theo delta sẽ trả rỗng và báo xanh trong
#   khi 90% đường dẫn đã chết (đúng chuyện đã xảy ra sau restructure 2026-07-28).
# DELTA (đắt, cần phán đoán): chỉ áp lên knowledge MỚI GHI kể từ mốc trước.
GATES=(verify-paths verify-links verify-symbols verify-frontmatter verify-depth verify-no-dup verify-orphan)

MODE=incremental
case "${1:-}" in
  --full)  MODE=full ;;
  --stamp) MODE=stamp ;;
  "")      ;;
  *) echo "tham số lạ: $1"; exit 2 ;;
esac

# ── mốc lần trước ───────────────────────────────────────────────────────────
read_watermark() { # <repo>
  [ -f "$STATE" ] || return 1
  python3 -c "
import json,sys
try: d=json.load(open('$STATE'))
except Exception: sys.exit(1)
r=d.get('runs') or []
if not r: sys.exit(1)
print(r[-1].get('watermark',{}).get('$1',''))" 2>/dev/null
}

sha_now() { git -C "$1" rev-parse HEAD 2>/dev/null; }

# bash 3.2 (mặc định macOS) KHÔNG có associative array — dùng hàm thay cho declare -A
repo_dir() {
  case "$1" in
    skill)      echo "$ROOT/skill" ;;
    frontend)   echo "$ROOT/frontend" ;;
    mobile)     echo "$ROOT/mobile" ;;
    erp-cluvix) echo "$ROOT" ;;
  esac
}

# ── TẦNG 1: FULL, luôn chạy ─────────────────────────────────────────────────
echo "── Tầng 1: gate FULL (luôn chạy, không phụ thuộc mốc) ──"
FAILED=0
for g in "${GATES[@]}"; do
  if out=$("$TOOLING/$g.sh" 2>&1); then
    printf "  ✅ %-20s %s\n" "$g" "$(tail -1 <<<"$out" | sed 's/^ *//')"
  else
    printf "  ❌ %-20s %s\n" "$g" "$(tail -2 <<<"$out" | head -1 | sed 's/^ *//')"
    FAILED=$((FAILED+1))
  fi
done
[ "$FAILED" -gt 0 ] && echo "  → $FAILED gate đỏ. Sửa trước khi làm gì khác."

# ── stamp ───────────────────────────────────────────────────────────────────
if [ "$MODE" = stamp ]; then
  if [ "$FAILED" -gt 0 ]; then
    echo; echo "⛔ TỪ CHỐI chốt mốc: còn $FAILED gate đỏ."
    echo "   Chốt mốc khi còn việc chưa xử = lần sau delta bắt đầu từ mốc mới và BỎ QUA"
    echo "   VĨNH VIỄN phần dở, không có gì báo ra. Đây là điều kiện chặn, không phải thủ tục."
    exit 1
  fi
  n=1; [ -f "$STATE" ] && n=$(python3 -c "import json;d=json.load(open('$STATE'));print(len(d.get('runs',[]))+1)" 2>/dev/null || echo 1)
  d=$(date +%Y-%m-%d)
  sk=$(sha_now "$(repo_dir skill)"); fe=$(sha_now "$(repo_dir frontend)")
  mb=$(sha_now "$(repo_dir mobile)"); er=$(sha_now "$(repo_dir erp-cluvix)")
  nsk=$(find "$ROOT"/{skill,frontend/skill,mobile/skill}/skills -maxdepth 1 -mindepth 1 -type d ! -name '_*' 2>/dev/null | wc -l | tr -d ' ')
  nln=$(find "$ROOT"/{skill,frontend/skill,mobile/skill} -name '*.md' -not -path '*/.git/*' 2>/dev/null | xargs cat 2>/dev/null | wc -l | tr -d ' ')
  python3 - "$STATE" "$n" "$d" "$MODE" "$sk" "$fe" "$mb" "$er" "$nsk" "$nln" <<'PY'
import json,sys,os
path,n,d,mode,sk,fe,mb,er,nsk,nln=sys.argv[1:11]
data={"runs":[]}
if os.path.exists(path):
    try: data=json.load(open(path))
    except Exception: pass
data.setdefault("runs",[]).append({
 "n":int(n),"date":d,"mode":"full",
 "watermark":{"skill":sk,"frontend":fe,"mobile":mb,"erp-cluvix":er},
 "counts":{"skills":int(nsk),"lines":int(nln)},
})
os.makedirs(os.path.dirname(path),exist_ok=True)
json.dump(data,open(path,"w"),indent=2,ensure_ascii=False)
print(f"✅ đã chốt mốc lần #{n} ({d}) — {nsk} skill / {nln} dòng")
PY
  exit 0
fi

# ── TẦNG 2: DELTA ───────────────────────────────────────────────────────────
echo; echo "── Tầng 2: delta (knowledge mới kể từ mốc trước) ──"
if [ "$MODE" = full ]; then
  echo "  mode=full → bỏ qua mốc, coi toàn bộ knowledge là cần soi lại."
  exit $([ "$FAILED" -gt 0 ] && echo 1 || echo 1)
fi

DELTA_TOTAL=0; MISSING_WM=0
for r in skill frontend mobile; do
  wm=$(read_watermark "$r" || true)
  dir="$(repo_dir "$r")"
  sub="skills"; [ "$r" != skill ] && sub="skill/skills"
  if [ -z "${wm:-}" ]; then
    echo "  ⚠ $r: chưa có mốc → chạy --full lần đầu"; MISSING_WM=1; continue
  fi
  if ! git -C "$dir" cat-file -e "$wm^{commit}" 2>/dev/null; then
    echo "  ⚠ $r: mốc $wm không tồn tại (rebase? clone lại?) → HẠ về full, KHÔNG đoán"; MISSING_WM=1; continue
  fi
  # Mốc so với WORKING TREE (bỏ "..HEAD") + file mới chưa track: knowledge vừa ghi
  # bằng /save-skill thường CHƯA commit — so với HEAD là bỏ lọt đúng phần mới nhất.
  list=$( { git -C "$dir" diff --name-only "$wm" -- "$sub" 2>/dev/null
            git -C "$dir" ls-files --others --exclude-standard -- "$sub" 2>/dev/null
          } | sort -u )
  files=$(printf '%s' "$list" | grep -c . || true)
  DELTA_TOTAL=$((DELTA_TOTAL+files))
  printf "  %-10s %3d file knowledge đổi kể từ %s (gồm cả chưa commit)\n" "$r" "$files" "${wm:0:8}"
  [ "$files" -gt 0 ] && printf '%s\n' "$list" | sed 's/^/       /'
done

echo
if [ "$MISSING_WM" = 1 ]; then
  echo "→ Thiếu/không tra được mốc. Chạy: $0 --full"; exit 1
fi
if [ "$DELTA_TOTAL" = 0 ] && [ "$FAILED" = 0 ]; then
  echo "✅ Không có knowledge mới, mọi gate sạch — không còn việc phải làm."
  exit 0
fi
if [ "$DELTA_TOTAL" -gt 0 ]; then
  cat <<'TXT'
→ Việc cần agent làm trên phần DELTA (không phải toàn bộ corpus):
   1. Xếp chỗ: mục mới có nằm đúng skill không (bảng phân loại: skill/HOW_TO_UPDATE.md).
   2. Dedup ĐỐI CHIẾU TOÀN CORPUS, không chỉ so delta với delta — trùng lặp điển hình là
      rule mới ghi lại đúng cái đã nằm trong file 6 tháng chưa ai đụng.
   3. Body SKILL.md vượt 500 dòng → tách xuống reference/.
   4. description còn nêu đủ "làm gì + KHI NÀO" sau khi thêm nội dung không.
   5. Test lại fresh-context: skill có file trong delta + skill đánh dấu `weak` + ngẫu nhiên ≥3 skill khác.
TXT
fi
exit 1
