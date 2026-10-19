#!/usr/bin/env bash
# verify-symbols.sh — kiểm SYMBOL (hàm/type) mà skill nhắc có TỒN TẠI TRONG CODE không.
#
# Vì sao cần (sự cố thật 2026-08-17): `verify-paths.sh` chỉ kiểm ĐƯỜNG DẪN tồn tại.
# Nó KHÔNG bắt được trường hợp path đúng mà thứ được mô tả không có thật. Ca đã xảy ra:
# `zns.md` mô tả `SendForAppointment(...)`/`SendManual(...)` với chữ ký đầy đủ — thư mục
# tồn tại, hàm thì CHƯA BAO GIỜ tồn tại. Không gate nào bắt được, phải đọc tay mới thấy.
# Ca thứ hai: doc ghi `exportHandlers.SetDownloadURL(url)` trong khi code đã đổi sang gán
# field `exportHandlers.DownloadURL = cfg.DownloadURL`.
#
# Đây là gate DUY NHẤT trả lời được "skill có khớp code không", chứ không chỉ "link có sống không".
#
# Dùng:  ./verify-symbols.sh [--self-test]
# Exit: 0 = khớp · 1 = có symbol không tồn tại · 2 = self-test trượt

set -uo pipefail
TOOLING="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="${CLUVIX_ROOT:-$(cd "$TOOLING/../../.." && pwd)}"
ALLOW="$TOOLING/verify-symbols-allow.txt"

run() {
python3 - "$1" "$2" <<'PY'
import re,pathlib,sys,collections
root=pathlib.Path(sys.argv[1]); allow_f=pathlib.Path(sys.argv[2])
allow=set()
if allow_f.exists():
    for l in allow_f.read_text().split('\n'):
        l=l.split('|')[0].strip()
        if l and not l.startswith('#'): allow.add(l)

# ⚠ Phải quét ĐỦ mọi gốc code. Thiếu 1 gốc = báo động giả hàng loạt.
#   (đã cắn: bỏ sót mobile/packages/ → MasterDataSync bị báo "không tồn tại" oan)
ROOTS=['backend','frontend/src','mobile/apps','mobile/packages']
exts={'.go','.ts','.dart','.html'}
idents=set(); scanned=0
for base in ROOTS:
    b=root/base
    if not b.is_dir(): continue
    for f in b.rglob('*'):
        if f.suffix in exts and f.is_file():
            try: idents.update(re.findall(r'\b[A-Za-z_][A-Za-z0-9_]{3,}\b', f.read_text(errors='ignore'))); scanned+=1
            except Exception: pass

pat=re.compile(r'`(?:[a-zA-Z_][\w]*\.)?([A-Z][A-Za-z0-9_]{3,})\(')
cand=collections.defaultdict(list)
for r in ['skill/skills','frontend/skill/skills','mobile/skill/skills']:
    d=root/r
    if not d.is_dir(): continue
    for f in d.rglob('*.md'):
        for i,l in enumerate(f.read_text().split('\n'),1):
            for m in pat.finditer(l):
                n=m.group(1)
                if n not in allow: cand[n].append(f"{f.relative_to(root)}:{i}")
miss={n:l for n,l in cand.items() if n not in idents}
print(f"{scanned}|{len(idents)}|{len(cand)}|{len(miss)}")
for n,l in sorted(miss.items()): print(f"{n}|{l[0]}")
PY
}

self_test() {
  local t; t=$(mktemp -d)
  mkdir -p "$t/backend" "$t/skill/skills/x"
  printf 'package p\nfunc ThisExists() {}\n' >"$t/backend/a.go"
  printf -- '---\nname: x\ndescription: d\n---\nDùng `ThisExists()` và `NeverExisted()`.\n' >"$t/skill/skills/x/SKILL.md"
  local out n; out=$(run "$t" /dev/null); n=$(head -1 <<<"$out" | cut -d'|' -f4)
  rm -rf "$t"
  echo "── self-test ──"
  if [ "$n" = "1" ]; then echo "  ✔ hàm có thật → bỏ qua · hàm bịa → bị bắt"; echo "  self-test PASS"; return 0; fi
  echo "  ✘ bắt được $n (cần đúng 1)"; echo "  self-test FAIL"; return 1
}

[ "${1:-}" = "--self-test" ] && { self_test; exit $?; }
self_test >/dev/null || { self_test; exit 2; }

OUT=$(run "$ROOT" "$ALLOW"); HDR=$(head -1 <<<"$OUT")
IFS='|' read -r NF NI NC NM <<<"$HDR"
echo "verify-symbols: quét $NF file code ($NI identifier) · skill nhắc $NC symbol · KHÔNG tồn tại: $NM"
if [ "$NM" -gt 0 ]; then
  tail -n +2 <<<"$OUT" | awk -F'|' '{printf "  %-32s %s\n",$1,$2}'
  echo
  echo "Symbol không tồn tại = skill mô tả API không có thật. Agent tin theo sẽ viết code gọi hàm ma."
  echo "Đúng thật mà bị báo oan (hàm built-in MySQL, placeholder Xxx...) → thêm vào verify-symbols-allow.txt KÈM LÝ DO."
  exit 1
fi
echo "  ✅ mọi symbol skill nhắc đều tồn tại trong code"
