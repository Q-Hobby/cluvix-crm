#!/usr/bin/env bash
# verify-id-params.sh — bắt path param đi THẲNG vào query mà chưa parse ra số.
#
# Vì sao cần script (đọc code không suy ra được): MySQL so cột INT với chuỗi bằng cách
# ÉP KIỂU chuỗi → số. `WHERE id = '79738abc'` khớp đúng dòng id=79738 và chỉ sinh
# warning 1292, GORM nuốt warning ⇒ build xanh, test xanh, request trả 200 "thành công".
# Trên đường GHI/XOÁ thì URL rác sửa/xoá trúng record thật.
# Đã kiểm qua đúng đường GORM + driver, không chỉ bằng `mysql` client (2026-09-07):
#   testDB.Table("customer").Where("id = ?", "1abc").Take(&m)  → trả về record id=1
# Rule đầy đủ: skill/skills/backend-hard-rules/SKILL.md §"Path param id phải parse ra số".
#
# Cơ chế quét: theo TỪNG HÀM (không theo file) — bắt 2 mẫu
#   (a) inline:  Where("id = ?", c.Param("id"))
#   (b) qua biến: id := c.Param("id")  … rồi  Where(... id ...)  trong cùng hàm
# Bỏ qua param mà kiểu chuỗi là ĐÚNG (token/code/slug…) — xem ALLOW_PARAM bên dưới.
#
# ⚠ Đây là gate CHỐNG TÁI DIỄN, không phải gate sạch-tuyệt-đối: repo còn tồn kho
# vi phạm cũ, nên script so với BASELINE và chỉ đỏ khi số vi phạm TĂNG. Vá bớt thì
# hạ BASELINE xuống theo (một người ghi, đừng để 2 phiên cùng cộng trừ con số này).
#
# Phủ CẢ path param (c.Param) LẪN query param tên id (c.Query/c.DefaultQuery) —
# cùng một cơ chế MySQL ép kiểu, chỉ khác cửa vào (mở rộng 2026-09-07).
# Dùng: verify-id-params.sh [--self-test] [--list]
# Exit: 0 = không tăng so với baseline · 1 = có vi phạm MỚI · 2 = self-test trượt

set -uo pipefail
ROOT="${CLUVIX_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)}"
BASELINE=0

scan() { # $1 = thư mục gốc cần quét
python3 - "$1" <<'PY'
import os,re,sys
root=sys.argv[1]
# Tên param mà giá trị CHUỖI là đúng nghiệp vụ — không phải khoá chính dạng số.
ALLOW_PARAM={'token','code','slug','key','handler_key','phone','tag','name'}
# c.Query() phục vụ cả filter/sort/paging, nên chỉ soi param có TÊN là id
# (id, idCustomer, customer_id, broker_id…) — đó là thứ đi vào `WHERE <cột int> = ?`.
IDISH=re.compile(r'(^|_)(id)$|^id',re.I)
# ⚠ Tên hàm Go: method có receiver `(h *Handler)` nên KHÔNG cắt bằng split("(")[0].
FUNC=re.compile(r'^func\s+(?:\([^)]*\)\s*)?(\w+)')
ASSIGN=re.compile(r'^\s*(\w+)\s*:?=\s*c\.(?:Param|Query|DefaultQuery)\("([^"]+)"[^)]*\)\s*$')
SINKS=r'Where|First|Take|Find|Exec|Raw|Updates|Update|Delete|Count|Scan|Pluck'
INLINE=re.compile(r'(?:'+SINKS+r')\([^)]*c\.(?:Param|Query|DefaultQuery)\("([^"]+)"[^)]*\)')
SINK=re.compile(r'\b(?:'+SINKS+r')\(')
WRITE=re.compile(r'\b(?:Updates|Update|Delete|Exec|Save)\(')
hits=[]
for dirpath,_,files in os.walk(root):
    if '/.git' in dirpath: continue
    for f in files:
        if not f.endswith('.go') or f.endswith('_test.go'): continue
        p=os.path.join(dirpath,f)
        try: lines=open(p,encoding='utf-8').read().split('\n')
        except Exception: continue
        fn=None; start=0; body=[]
        def flush(name,st,bd):
            if not name: return
            raw={}
            for l in bd:
                m=ASSIGN.match(l)
                if m: raw[m.group(1)]=m.group(2)
            for i,l in enumerate(bd):
                if l.lstrip().startswith('//'): continue      # comment chứa từ khoá = hit giả
                m=INLINE.search(l)
                if m:
                    if m.group(1) not in ALLOW_PARAM and IDISH.search(m.group(1)):
                        hits.append((p,st+i+1,name,m.group(1),bool(WRITE.search(l))))
                    continue
                if not SINK.search(l) or 'c.Param' in l or 'c.Query' in l: continue
                for v,pn in raw.items():
                    if pn in ALLOW_PARAM or not IDISH.search(pn): continue
                    if re.search(r'[,(\s]'+re.escape(v)+r'\b',l):
                        hits.append((p,st+i+1,name,pn,bool(WRITE.search(l)))); break
        for i,l in enumerate(lines):
            m=FUNC.match(l)
            if m: flush(fn,start,body); fn=m.group(1); start=i; body=[]
            else: body.append(l)
        flush(fn,start,body)
seen=set(); out=[]
for h in hits:
    if (h[0],h[1]) in seen: continue
    seen.add((h[0],h[1])); out.append(h)
for p,ln,fn,pn,w in sorted(out):
    print(f"{'WRITE' if w else 'read '}\t{p}:{ln}\t{fn}\tparam={pn}")
PY
}

self_test() {
  local t; t=$(mktemp -d); trap 'rm -rf "$t"' RETURN
  cat > "$t/bad.go" <<'EOF'
package h
func (h *Handler) BadInline(c *gin.Context) {
	db.Table("x").Where("id = ?", c.Param("id")).Take(&m)
}
func (h *Handler) BadVar(c *gin.Context) {
	id := c.Param("id")
	db.Table("x").Where("id = ?", id).Delete(nil)
}
func (h *Handler) BadQueryVar(c *gin.Context) {
	customerID := c.Query("idCustomer")
	db.Table("x").Where("customer_id = ?", customerID).Find(&m)
}
func (h *Handler) BadQueryInline(c *gin.Context) {
	db.Table("x").Where("broker_id = ?", c.Query("broker_id")).Find(&m)
}
EOF
  cat > "$t/good.go" <<'EOF'
package h
func (h *Handler) GoodParsed(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok { return }
	db.Table("x").Where("id = ?", id).Take(&m)
}
func (h *Handler) GoodStringParam(c *gin.Context) {
	db.Table("x").Where("token = ?", c.Param("token")).Take(&m)
}
func (h *Handler) GoodQueryFilter(c *gin.Context) {
	// query param KHÔNG phải id (filter/sort/paging) — chuỗi là đúng, không được báo
	db.Table("x").Where("status = ?", c.Query("status")).Find(&m)
}
func (h *Handler) GoodQueryParsed(c *gin.Context) {
	customerID, _ := strconv.ParseUint(c.Query("customer_id"), 10, 64)
	db.Table("x").Where("customer_id = ?", customerID).Find(&m)
}
func (h *Handler) GoodComment(c *gin.Context) {
	// Where("id = ?", c.Param("id")) — mẫu SAI, chỉ là ví dụ trong comment
	id, _ := helpers.ParseID(c)
	db.Table("x").Where("id = ?", id).Take(&m)
}
EOF
  local bad good names
  bad=$(scan "$t" | grep -c "/bad.go" || true)
  good=$(scan "$t" | grep -c "/good.go" || true)
  names=$(scan "$t" | awk -F'\t' '{print $3}' | grep -c "^Bad" || true)
  echo "── self-test ──"
  echo "  mẫu SAI bắt được: $bad/4 · mẫu ĐÚNG báo nhầm: $good (cần 0) · tên hàm parse đúng: $names/4"
  if [ "$bad" = 4 ] && [ "$good" = 0 ] && [ "$names" = 4 ]; then
    echo "  self-test PASS"; return 0
  fi
  echo "  self-test FAIL — scanner hỏng, MỌI con số dưới đây vô nghĩa"; return 1
}

case "${1:-}" in
  --self-test) self_test; exit $? ;;
esac
self_test >/dev/null 2>&1 || { self_test; exit 2; }

TMP=$(mktemp); trap 'rm -f "$TMP"' EXIT
scan "$ROOT/backend/internal" >"$TMP"
N=$(wc -l <"$TMP" | tr -d ' ')
W=$(grep -c '^WRITE' "$TMP" || true)
FILES=$(awk -F'\t' '{split($2,a,":"); print a[1]}' "$TMP" | sort -u | wc -l | tr -d ' ')

[ "${1:-}" = "--list" ] && sed "s|$ROOT/||" "$TMP"

echo "verify-id-params:"
echo "  $N điểm / $FILES file · $W điểm nằm trên đường GHI/XOÁ · baseline = $BASELINE"
if [ "$N" -le "$BASELINE" ]; then
  [ "$N" -lt "$BASELINE" ] && echo "  ℹ️  thấp hơn baseline — vá xong thì hạ BASELINE trong script này xuống $N"
  echo "  ✅ không có vi phạm MỚI (dùng --list để xem toàn bộ tồn kho)"
  exit 0
fi
echo "  ✘ TĂNG $((N-BASELINE)) điểm so với baseline — có handler mới đưa c.Param() thẳng vào query"
echo "     Sửa: parse ra số trước (helpers.ParseID), xem backend-hard-rules §Path param id."
exit 1
