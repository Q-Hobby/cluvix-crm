#!/usr/bin/env python3
"""body-id-scan.py — quét handler GHI DB theo khoá chính NẰM TRONG STRUCT, sau khi để client bind
đè lên chính struct đó ("mass-assignment khoá chính" / IDOR-ghi qua body).

Lỗ (epic `idor-body-id-write`, phát hiện 2026-08-12 — `skill/bmad/idor-body-id-write/architecture.md`):

    tenant.ScopedDBNullBranch(c, h.DB, "t").First(&item, id)   // ĐỌC có scope — đúng
    c.ShouldBindJSON(&item)                                    // client ghi đè được item.ID
    h.DB.Save(&item)                                           // GHI theo item.ID — KHÔNG scope

`Save()` ghi theo khoá chính trong struct. Client gửi `{"id": 99}` (lane JSON) hoặc form field tên `ID`
(lane multipart — gin form-binding KHÔNG đọc tag `json`, thiếu tag `form` thì nó lấy TÊN FIELD làm key)
là request sửa sang bản ghi 99: toàn bộ lớp tenant scope ở bước `First()` bị vô hiệu, route `:id` chỉ
còn tác dụng chọn bản ghi để ĐỌC. Lỗ này KHÁC lớp mà `scripts/idor-scan.py` canh (id từ route/query +
thiếu scope): ở đây scope ĐÚNG, id đến từ BODY.

Đã vá bằng `helpers.BindUpdate` / `helpers.BindUpdateWith` (`pkg/helpers/bind_update.go`) — helper ghim
`ID` (luôn luôn) + các field caller liệt kê, bằng snapshot DEEP-COPY trước bind. Script coi việc dùng
helper là PASS; đó là ngưỡng "đã bảo vệ" duy nhất được chấp nhận cho code MỚI.

Rule tương ứng: `skill/backend/rules/backend.md` mục "Handler seed→bind→`Save()`".

⚠ ĐỌC TRƯỚC KHI TIN KẾT QUẢ — vì sao có `SELF_TEST`:
    `scripts/idor-scan.py` (script IDOR đời đầu của repo) từng báo "sạch" trong khi bug thật nằm NGAY
    trong phạm vi quét, chỉ vì một luật "đã bảo vệ" sai. Lớp lỗ này còn dễ tự lừa hơn: 3 script quét tạm
    của spike story-01 đã (a) đếm 86 trong khi số đúng là 81, (b) bỏ sót toàn bộ lane multipart,
    (c) bỏ sót dạng `ShouldBindJSON(item)` KHÔNG có `&`, (d) coi "có dòng gán lại" là an toàn trong khi
    pin field CON TRỎ hoàn toàn vô hiệu. Vì vậy script LUÔN tự kiểm trên code THẬT ở trạng thái TRƯỚC
    KHI VÁ (fixture nhúng, xem `SELF_TEST`); self-test fail ⇒ exit 2 và KHÔNG in kết quả quét.
    Sửa bất kỳ luật nào bên dưới ⇒ phải chạy lại self-test.

Vì sao fixture NHÚNG chứ không `git show <commit>:<path>` (khác `idor-scan.py`):
    Bản vá của epic này còn NẰM TRONG WORKING TREE lúc script ra đời (chưa commit) — không có SHA "bản
    vá" để lấy `<sha>^`. Fixture được trích NGUYÊN VĂN từ `git show f65fe817:<path>` (HEAD lúc đó = code
    chưa vá) và nhúng thẳng vào script, nên self-test không phụ thuộc lịch sử git (rebase/squash/rewrite
    không làm hỏng), không phụ thuộc thứ tự commit, và chạy được cả trong tarball không có `.git`.
    Đổi lại: fixture là ẢNH TĨNH — nó chứng minh LUẬT còn sống, không chứng minh code hiện tại sạch.

Phán đoán của script (mỗi finding = 1 trong 4 loại):
    ID-UNPINNED  bind thô vào X rồi `Save(X)` mà giữa 2 chỗ KHÔNG có gì ghim `X.ID`      → FAIL
    ID-PIN-COND  có ghim `X.ID` nhưng nằm trong nhánh `if`/block ⇒ không phải mọi đường  → FAIL
    PIN-PTR      pin tay field CON TRỎ (`orig := X.F` … `X.F = orig`, `F` kiểu `*T`)     → FAIL
                 `encoding/json` ghi XUYÊN QUA con trỏ non-nil ⇒ biến "giữ" chính là ô nhớ vừa bị
                 payload sửa; gán ngược không khôi phục gì (gotcha story-02 §6, story-03 §3).
    PIN-PTR-ARG  con trỏ chụp trước bind KHÔNG restore mà truyền vào HÀM khác
                 (`AuditUpdatedBy(uid, origUpdatedBy)` — story-06 §3)                    → WARN
    PIN-ALIAS    `body := struct{ dao.X; … }{X: item}` → bind vào `body` → "khôi phục"
                 `body.F = item.F`. Struct được COPY nhưng field CON TRỎ vẫn trỏ CHUNG ô
                 nhớ với `item` ⇒ payload đã ghi xuyên qua CẢ HAI, khôi phục vô nghĩa.
                 (biến thể B của epic; đã chứng minh bằng chương trình Go tối giản)       → WARN
    CREATE-PK    bind entity rồi `Create` mà không `X.ID = 0` ⇒ client tự chọn PK         → WARN
    BIND-DST-EXPR bind vào biểu thức không phải biến đơn (`&body.Details`, struct ẩn danh
                 dựng tại chỗ) — script không lần được, đọc tay                            → WARN

Được coi là AN TOÀN (không báo đỏ) — các khuôn đã xác nhận tay ở story-03/04:
    - `helpers.BindUpdate` / `BindUpdateWith` trên đúng biến đó (bất kể ở nhánh nào của hàm).
    - Bind vào DTO/struct RIÊNG rồi copy field tường minh sang entity (`unit`, `options`,
      `print_template`, `service_type`, `accounting_period`…): biến bind ≠ biến `Save` ⇒ không xét.
    - Bind vào `map[string]interface{}` rồi merge từng key (`config/company`, `client/sync/*`).
    - DTO ẩn danh tự khai `ID` cho toggle/bulk-sort (21 ca `ANON-HAS-ID`): id-từ-body HỢP LỆ, và biến
      bind cũng ≠ biến `Save`.
    - Pin tay `X.ID = …` (kể cả **gán tuple** `a, b := X.ID, X.Y` … `X.ID, X.Y = a, b`) khi field là
      KIỂU GIÁ TRỊ và pin vô điều kiện.

GIỚI HẠN ĐÃ BIẾT (đọc trước khi coi "0 finding" là "0 lỗ"):
    1. Chỉ quét file handler trong `internal/modules/**` (`/handler/` hoặc tên file `handler*.go`).
       Bind + `Save` nằm trong `service/`, `cmd/`, cron → KHÔNG quét.
    2. Chỉ bắt khi biến bind và biến `Save`/`Updates` là CÙNG một identifier trong CÙNG một hàm. Bind ở
       hàm A rồi `Save` ở hàm B (truyền con trỏ qua tham số) → KHÔNG bắt.
    3. `X.ID` được ghim bằng đường gián tiếp (gọi hàm helper riêng của module, `reflect`, gán qua con
       trỏ trung gian) → script không thấy ⇒ FALSE POSITIVE. Đọc tay rồi cho vào `BASELINE` kèm lý do.
    4. Phân giải kiểu field theo TÊN KIỂU toàn repo (không theo import path). Trùng tên kiểu giữa 2
       package ⇒ đánh dấu ambiguous và in ở mục REVIEW thay vì đoán bừa.
    5. `PIN-PTR` chỉ xét field được restore từ biến chụp TRƯỚC bind. Pin bằng cách khác (deep-copy tay,
       gán hằng) → không xét. Field không phân giải được kiểu → mục REVIEW, không FAIL.
    6. Không mô hình hoá luồng: `Save` trong nhánh `else` mà pin ở nhánh `if` vẫn tính là ID-PIN-COND
       (cố ý — "có dòng restore" ≠ PASS).
    7. Lane CREATE (`bind entity` → `Create` mà không `ID = 0`, client tự chọn PK) chỉ in ở mục WARN,
       không FAIL — mức thấp, không phải IDOR ghi đè.
    8. `PIN-ALIAS` là WARN chứ không FAIL vì tác động phụ thuộc field (audit/guard, không phải `id`)
       và vì field NULL trong DB (con trỏ `nil`) thì `json` cấp phát mới ⇒ khôi phục vẫn đúng. Chỉ
       bản ghi có giá trị non-nil mới khai thác được.
    9. Không đọc tag `form:"-"`: phòng thủ chiều sâu ở tầng entity (story-05) KHÔNG được tính là "đã
       bảo vệ" — helper mới là chốt chặn, tag chỉ là lưới thứ 2.

Exit code: 0 = sạch · 1 = có finding ngoài BASELINE · 2 = self-test fail (script hỏng, kết quả vô nghĩa).

Dùng: python3 scripts/body-id-scan.py [--all] [--warn]
      --all   in cả mục đã nằm trong BASELINE
      --warn  in đầy đủ mục WARN/REVIEW (mặc định chỉ in số lượng + 5 dòng đầu)
"""
import os
import re
import sys

BE = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
MODULES = os.path.join(BE, 'internal/modules')

# ---------------------------------------------------------------- Baseline
# Khoá = "<func>@<đuôi path>". Chỉ thêm sau khi ĐỌC TAY code, kèm lý do. KHÔNG thêm cho hết đỏ.
BASELINE = {
    # (trống — sau story-02..07 toàn bộ 81+ điểm đã chuyển sang helpers.BindUpdate)
}


# ---------------------------------------------------------------- tiện ích đọc Go
def strip_comments(src):
    """Bỏ comment `//` và `/* */`, GIỮ nguyên độ dài chuỗi (thay bằng space) để offset/dòng không lệch.
    Tôn trọng string literal — nếu không, `"https://x"` sẽ nuốt phần còn lại của dòng."""
    out = list(src)
    i, n = 0, len(src)
    while i < n:
        ch = src[i]
        if ch == '"' or ch == '`' or ch == "'":
            q = ch
            i += 1
            while i < n and src[i] != q:
                if q != '`' and src[i] == '\\':
                    i += 1
                i += 1
            i += 1
            continue
        if ch == '/' and i + 1 < n and src[i + 1] == '/':
            while i < n and src[i] != '\n':
                out[i] = ' '
                i += 1
            continue
        if ch == '/' and i + 1 < n and src[i + 1] == '*':
            while i < n and not (src[i] == '*' and i + 1 < n and src[i + 1] == '/'):
                if src[i] != '\n':
                    out[i] = ' '
                i += 1
            if i < n:
                out[i] = out[i + 1] = ' '
                i += 2
            continue
        i += 1
    return ''.join(out)


FUNC_RE = re.compile(r'^func (?:\([^)]*\)\s*)?(\w+)\(', re.M)


def funcs(src):
    """(tên, body, dòng bắt đầu). Body = tới `func` kế tiếp — đủ dùng, Go không lồng func top-level."""
    src = strip_comments(src)
    ms = list(FUNC_RE.finditer(src))
    for k, m in enumerate(ms):
        end = ms[k + 1].start() if k + 1 < len(ms) else len(src)
        yield m.group(1), src[m.start():end], src[:m.start()].count('\n') + 1


def split_args(s, open_idx):
    """Tách đối số ở mức ngoặc ngoài cùng. `s[open_idx]` phải là '('. Trả (list_arg, idx_đóng)."""
    depth, args, cur, i = 0, [], [], open_idx
    n = len(s)
    while i < n:
        ch = s[i]
        if ch in '([{':
            depth += 1
            if depth == 1:
                i += 1
                continue
        elif ch in ')]}':
            depth -= 1
            if depth == 0:
                args.append(''.join(cur).strip())
                return [a for a in args if a], i
        elif ch == ',' and depth == 1:
            args.append(''.join(cur).strip())
            cur = []
            i += 1
            continue
        elif ch in '"`':
            q = ch
            cur.append(ch)
            i += 1
            while i < n and s[i] != q:
                cur.append(s[i])
                i += 1
        if depth >= 1:
            cur.append(s[i])
        i += 1
    return [], -1


IDENT = re.compile(r'^&?\*?([A-Za-z_]\w*)$')


def arg_ident(a):
    """`&item` / `item` / `*cfg` → 'item'/'cfg'; biểu thức phức tạp (`&input.Foo`) → None."""
    m = IDENT.match(a.strip())
    return m.group(1) if m else None


def depth_at(body, pos):
    """Độ sâu ngoặc nhọn tại `pos` tính từ đầu hàm (1 = thân hàm, >1 = trong block con)."""
    return body.count('{', 0, pos) - body.count('}', 0, pos)


# ---------------------------------------------------------------- index struct toàn repo
_STRUCTS = {}      # tên kiểu -> {'fields': {tên: kiểu}, 'embeds': [tên], 'amb': bool}
TYPE_RE = re.compile(r'^type (\w+) struct \{', re.M)


def _load_structs():
    if _STRUCTS:
        return
    for root in ('internal', 'pkg'):
        for dp, _, fs in os.walk(os.path.join(BE, root)):
            for fn in fs:
                if not fn.endswith('.go') or fn.endswith('_test.go'):
                    continue
                src = strip_comments(open(os.path.join(dp, fn), encoding='utf-8', errors='replace').read())
                for m in TYPE_RE.finditer(src):
                    body, d, i = [], 1, m.end()
                    while i < len(src) and d:
                        if src[i] == '{':
                            d += 1
                        elif src[i] == '}':
                            d -= 1
                            if not d:
                                break
                        body.append(src[i])
                        i += 1
                    info = _parse_fields(''.join(body))
                    name = m.group(1)
                    prev = _STRUCTS.get(name)
                    if prev and (prev['fields'] != info['fields'] or prev['embeds'] != info['embeds']):
                        prev['amb'] = True          # trùng tên kiểu ở 2 package, khác shape
                    elif not prev:
                        _STRUCTS[name] = info


def _parse_fields(body):
    fields, embeds = {}, []
    for raw in body.split('\n'):
        line = re.sub(r'`[^`]*`', '', raw).strip()          # bỏ tag
        if not line or line.startswith(('//', '}', '{')):
            continue
        toks = line.split()
        if len(toks) == 1:                                   # embedded: `models.BaseModel`
            embeds.append(toks[0].split('.')[-1])
            continue
        names, typ = toks[0], toks[1]
        if names.endswith(','):                              # `A, B uint`
            names = ' '.join(toks[:-1])
            typ = toks[-1]
        for nm in [x.strip() for x in names.split(',') if x.strip()]:
            if re.match(r'^[A-Za-z_]\w*$', nm):
                fields[nm] = typ
    return {'fields': fields, 'embeds': embeds, 'amb': False}


def field_type(type_name, field, _seen=None):
    """Kiểu khai báo của `type_name.field`, đi xuyên struct nhúng. None = không phân giải được.
    Trả ('?', True) khi tên kiểu bị trùng giữa 2 package ⇒ caller đẩy sang mục REVIEW."""
    _load_structs()
    _seen = _seen or set()
    if type_name in _seen or type_name not in _STRUCTS:
        return None
    _seen.add(type_name)
    info = _STRUCTS[type_name]
    if info['amb']:
        return '?'
    if field in info['fields']:
        return info['fields'][field]
    for e in info['embeds']:
        t = field_type(e, field, _seen)
        if t:
            return t
    return None


def is_pointer(typ):
    return bool(typ) and typ != '?' and typ.startswith('*')


# ---------------------------------------------------------------- luật quét
# bind THÔ = client ghi thẳng vào biến đích, không qua chốt chặn
RAW_BIND = re.compile(
    r'c\.ShouldBind\w*\s*(?=\()'                 # ShouldBindJSON/ShouldBind/ShouldBindWith…
    r'|c\.Bind\w*\s*(?=\()'
    r'|json\.Unmarshal\s*(?=\()'
    r'|\bbind[A-Z]\w*\s*(?=\()'                  # binder multipart thủ công: bindBranchFormFields(c, &item)
)
SAFE_BIND = re.compile(r'helpers\.BindUpdate(?:With)?\s*(?=\()')
WRITE_BY_PK = re.compile(r'\.Save\s*(?=\()|\.Updates\s*\(\s*&')
CREATE_CALL = re.compile(r'\.Create\s*(?=\()')


def _calls(body, pat):
    """[(tên_gọi_bắt_đầu, args, pos)] cho mọi lời gọi khớp `pat`."""
    out = []
    for m in pat.finditer(body):
        if re.search(r'\bfunc\s+$', body[:m.start()]):
            continue                                  # KHAI BÁO hàm `func bindXxxFormFields(...)`
        args, close = split_args(body, m.end())
        if close > 0:
            out.append((m.start(), args))
    return out


def _captures(body, upto):
    """Biến chụp giá trị field TRƯỚC bind: `a, b := X.F1, X.F2` → {'a': ('X','F1'), 'b': ('X','F2')}."""
    caps = {}
    for line in body[:upto].split('\n'):
        if ':=' not in line:
            continue
        lhs, rhs = line.split(':=', 1)
        ls = [x.strip() for x in lhs.split(',')]
        rs = [x.strip() for x in rhs.split(',')]
        if len(ls) != len(rs):
            continue
        for lv, rv in zip(ls, rs):
            m = re.match(r'^([A-Za-z_]\w*)\.([A-Za-z_]\w*)$', rv)
            if m and re.match(r'^[A-Za-z_]\w*$', lv):
                caps[lv] = (m.group(1), m.group(2))
    return caps


def _lhs_fields(segment, var):
    """[(field, pos, depth_line)] cho mọi gán có `var.Field` ở VẾ TRÁI (kể cả gán tuple)."""
    out = []
    for m in re.finditer(r'^(?P<lhs>[^\n=]*?)(?<![=!<>:+\-*/%&|^])=(?!=)(?P<rhs>[^\n]*)$', segment, re.M):
        lhs, rhs = m.group('lhs'), m.group('rhs')
        lparts = [x.strip() for x in lhs.split(',')]
        rparts = [x.strip() for x in rhs.split(',')]
        for i, lp in enumerate(lparts):
            f = re.match(r'^' + re.escape(var) + r'\.([A-Za-z_]\w*)$', lp)
            if f:
                out.append((f.group(1), m.start(), rparts[i] if i < len(rparts) else ''))
    return out


def resolve_var_type(body, var):
    """Kiểu khai báo của biến bind. Trả (nhãn, tên_kiểu|None)."""
    m = re.search(r'\bvar\s+' + re.escape(var) + r'\s+([\w.\[\]*]+)', body)
    if m:
        t = m.group(1)
        if t.startswith('map['):
            return 'MAP', None
        return 'ENTITY', t.split('.')[-1].lstrip('*')
    m = re.search(r'\b' + re.escape(var) + r'\s*(?:,\s*\w+\s*)?:=\s*([^\n]+)', body)
    if m:
        rhs = m.group(1)
        if rhs.startswith('map[') or 'make(map[' in rhs:
            return 'MAP', None
        if re.match(r'^\s*struct\s*\{', rhs):
            return ('ANON-HAS-ID' if re.search(r'\bID\s+\w', rhs) else 'ANON'), None
        # `item, err := repo.FindByID(...)` — kiểu suy từ repository.New[pkg.Entity]
        g = re.search(r'repository\.New\[[\w.]*?(\w+)\]', body)
        if g:
            return 'ENTITY', g.group(1)
        g = re.search(r'\b[\w.]*?(\w+)\{', rhs)
        if g:
            return 'ENTITY', g.group(1)
        return 'UNRESOLVED', None
    m = re.search(r'\bvar\s+' + re.escape(var) + r'\s*=\s*([^\n]+)', body)
    if m:
        return 'UNRESOLVED', None
    return 'UNRESOLVED', None


def scan_alias_pin(name, body, bound_vars):
    """PIN-ALIAS — `body := struct{ dao.X; … }{X: item}` rồi bind vào `body`, xong "khôi phục"
    `body.F = item.F`. Struct được COPY nhưng field CON TRỎ vẫn trỏ CHUNG ô nhớ với `item`, nên
    payload đã ghi xuyên qua cả hai; gán ngược không khôi phục gì. Cùng bản chất PIN-PTR nhưng
    nguồn khôi phục là BIẾN KHÁC (ca `med/stock_import`, bind nằm ở hàm khác nên luật chính không
    nhìn thấy)."""
    warns = []
    for m in re.finditer(r'^\s*(\w+)\.(?:\w+\.)?([A-Za-z_]\w*)\s*=\s*(\w+)\.\2\s*$', body, re.M):
        dst, fld, src = m.group(1), m.group(2), m.group(3)
        if dst == src or fld == 'ID':
            continue
        if src in bound_vars:
            continue        # `src` mới là biến nhận payload ⇒ đây là copy DTO→entity, không phải pin
        decl = re.search(r'\b' + re.escape(dst) + r'\s*:?=\s*((?:[^\n]|\n\s*[^\n])*)', body)
        if not decl or not re.search(r'\b' + re.escape(src) + r'\b', decl.group(1)):
            continue                                   # `dst` không được seed từ `src` ⇒ copy DTO thường
        _k, tname = resolve_var_type(body, src)
        ft = field_type(tname, fld) if tname else None
        if is_pointer(ft):
            warns.append(('PIN-ALIAS',
                          f'{name}: `{dst}` seed từ `{src}` rồi "khôi phục" `{dst}.{fld} = {src}.{fld}` '
                          f'({ft}) — 2 biến DÙNG CHUNG con trỏ, khôi phục VÔ HIỆU'))
    return warns


def scan_func(name, body, line):
    """Trả (findings, warns) cho 1 hàm. Mỗi phần tử: (loại, mô tả)."""
    bound_vars = set()
    for _pos, args in _calls(body, RAW_BIND):
        for a in reversed(args):
            v = arg_ident(a)
            if v and v not in ('c', 'err'):
                bound_vars.add(v)
                break
    findings, warns = [], scan_alias_pin(name, body, bound_vars)

    safe_vars = set()
    for pos, args in _calls(body, SAFE_BIND):
        for a in args:
            v = arg_ident(a)
            if v:
                safe_vars.add(v)
                break

    write_vars = {}     # var -> pos đầu tiên
    for pos, args in _calls(body, WRITE_BY_PK):
        if not args:
            continue
        v = arg_ident(args[0])
        if v:
            write_vars.setdefault(v, pos)

    create_vars = {}
    for pos, args in _calls(body, CREATE_CALL):
        if args:
            v = arg_ident(args[0])
            if v:
                create_vars.setdefault(v, pos)

    for pos, args in _calls(body, RAW_BIND):
        # đích bind = đối số cuối cùng dạng identifier (ShouldBindJSON(&item) / bindXxx(c,&item)
        # / json.Unmarshal(data,&item)); dạng `ShouldBindJSON(item)` KHÔNG có `&` cũng vào đây.
        dst = None
        for a in reversed(args):
            dst = arg_ident(a)
            if dst and dst not in ('c', 'err'):
                break
            dst = None
        if not dst:
            if args:
                warns.append(('BIND-DST-EXPR', f'{name}: bind vào biểu thức `{args[-1]}` — đọc tay'))
            continue
        if dst in safe_vars:
            continue                                   # đã qua helpers.BindUpdate*

        kind, tname = resolve_var_type(body, dst)
        if kind == 'MAP':
            continue                                   # lane map-merge — an toàn (story-01 AC4)

        if dst in create_vars and dst not in write_vars:
            if not any(f == 'ID' and rhs.strip() == '0' for f, _, rhs in _lhs_fields(body, dst)):
                warns.append(('CREATE-PK', f'{name}: bind entity `{dst}` rồi Create mà không `{dst}.ID = 0` '
                                           f'⇒ client tự chọn PK'))
            continue
        if dst not in write_vars:
            continue                                   # bind DTO rồi copy tường minh — khuôn AN TOÀN
        wpos = write_vars[dst]
        if wpos < pos:
            continue

        seg = body[pos:wpos]
        pins = [(f, p, rhs) for f, p, rhs in _lhs_fields(seg, dst)]
        id_pins = [(f, p) for f, p, _ in pins if f == 'ID']
        if not id_pins:
            findings.append(('ID-UNPINNED',
                             f'{name}: bind thô vào `{dst}` ({kind}{"/" + tname if tname else ""}) rồi '
                             f'ghi theo `{dst}.ID` — không có gì ghim ID'))
        elif all(depth_at(body, pos + p) > 1 for _, p in id_pins):
            findings.append(('ID-PIN-COND',
                             f'{name}: `{dst}.ID` chỉ được ghim TRONG NHÁNH — không phủ mọi đường ghi'))

        # --- pin tay field con trỏ (kể cả khi ID đã pin đúng) ---
        caps = _captures(body, pos)
        restored = set()
        for f, p, rhs in pins:
            if f == 'ID':
                continue
            src = caps.get(rhs.strip())
            if not src or src != (dst, f):
                continue                                # gán nghiệp vụ bình thường, không phải pin
            restored.add(rhs.strip())
            ft = field_type(tname, f) if tname else None
            if is_pointer(ft):
                findings.append(('PIN-PTR',
                                 f'{name}: pin tay `{dst}.{f}` kiểu `{ft}` — CON TRỎ, json ghi xuyên qua '
                                 f'⇒ pin VÔ HIỆU (dùng helpers.BindUpdate(c, &{dst}, "{f}"))'))
            elif ft is None or ft == '?':
                warns.append(('PIN-TYPE-UNKNOWN',
                              f'{name}: pin tay `{dst}.{f}` — không phân giải được kiểu '
                              f'({tname or "kiểu ẩn"}), ĐỌC TAY xem có phải con trỏ không'))
            elif depth_at(body, pos + p) > 1:
                warns.append(('PIN-COND', f'{name}: pin `{dst}.{f}` nằm trong nhánh `if` — có điều kiện'))
        # --- con trỏ chụp trước bind, KHÔNG restore mà truyền vào hàm khác (story-06) ---
        for cv, (cvar, cf) in caps.items():
            if cvar != dst or cv in restored or cf == 'ID':
                continue
            if not re.search(r'\w+\([^)]*\b' + re.escape(cv) + r'\b[^)]*\)', body[pos:]):
                continue
            ft = field_type(tname, cf) if tname else None
            if is_pointer(ft) or ft in (None, '?'):
                warns.append(('PIN-PTR-ARG',
                              f'{name}: `{cv}` chụp `{dst}.{cf}` ({ft or "kiểu ?"}) TRƯỚC bind rồi truyền '
                              f'vào hàm khác — nếu là con trỏ thì giá trị đã bị payload sửa'))
    return findings, warns


def iter_handlers():
    for dp, _, fs in os.walk(MODULES):
        for fn in sorted(fs):
            if not fn.endswith('.go') or fn.endswith('_test.go'):
                continue
            p = os.path.join(dp, fn)
            if '/handler/' not in p and not fn.startswith('handler'):
                continue
            yield p, open(p, encoding='utf-8', errors='replace').read()


# ---------------------------------------------------------------- self-test
# Fixture = code THẬT ở `git show f65fe817:<path>` (HEAD 2026-08-12, TRƯỚC khi epic vá). Nhúng thẳng
# để self-test không phụ thuộc lịch sử git (xem docblock). Rút gọn phần không liên quan bằng `…` là
# KHÔNG được phép — mọi fixture dưới đây là nguyên văn hàm.
FIX_EMR_BED = '''
func (h *Handler) UpdateEmrBed(c *gin.Context) {
	var item emrConfigDao.EmrBed
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	if err := tenant.ScopedDBNullBranch(c, h.DB, "emr_bed").First(&item, id).Error; err != nil {
		response.NotFound(c)
		return
	}
	oldVersion := item.Version
	lockedCompanyID, lockedBranchID := item.CompanyID, item.BranchId
	if err := c.ShouldBindJSON(&item); err != nil {
		response.HandleError(c, err)
		return
	}
	item.CompanyID, item.BranchId = lockedCompanyID, lockedBranchID
	item.Version = oldVersion + 1
	res := h.DB.Where("version = ?", oldVersion).Save(&item)
	if res.Error != nil {
		response.ServerError(c, res.Error)
		return
	}
	response.Updated(c, item)
}
'''

FIX_BRANCH = '''
func (h *Handler) UpdateBranch(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	var item branchDao.Branch
	if err := tenant.JSONCompanyScope(c, h.DB, "companyId").First(&item, id).Error; err != nil {
		response.NotFound(c)
		return
	}
	oldImage := item.Image
	contentType := c.ContentType()
	if strings.Contains(contentType, "multipart") {
		bindBranchFormFields(c, &item)
	} else {
		if err := c.ShouldBindJSON(&item); err != nil {
			response.HandleError(c, err)
			return
		}
	}
	now := time.Now().Unix()
	uid := int(middleware.GetUserID(c))
	item.UpdatedAt = now
	item.UpdatedBy = &uid
	if err := h.DB.Save(&item).Error; err != nil {
		response.ServerError(c, err)
		return
	}
	response.Updated(c, item)
}
'''

FIX_TAX_RATE = '''
func (h *Handler) Update(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	var item dao.TaxRate
	if err := tenant.ScopedDBNullBranch(c, h.DB, "tax_rate").First(&item, id).Error; err != nil {
		response.NotFound(c)
		return
	}
	if item.CompanyID == nil && !helpers.IsSuperUser(h.DB, middleware.GetUserID(c)) {
		response.Forbidden(c)
		return
	}
	origID, origCompany := item.ID, item.CompanyID
	if err := c.ShouldBindJSON(&item); err != nil {
		response.HandleError(c, err)
		return
	}
	item.ID, item.CompanyID = origID, origCompany
	if err := h.DB.Set("userID", middleware.GetUserID(c)).Save(&item).Error; err != nil {
		response.ServerError(c, err)
		return
	}
	response.Updated(c, item)
}
'''

# `repository.FindByID` trả CON TRỎ ⇒ bind là `ShouldBindJSON(item)` KHÔNG có `&`.
# Script spike story-01 bỏ sót nguyên lớp này (story-03 phát hiện).
FIX_EMR_DEPARTMENT = '''
func (h *Handler) UpdateEmrDepartment(c *gin.Context) {
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	repo := repository.New[emrConfigDao.EmrDepartment](h.DB, "emr_department")
	item, err := repo.FindByID(c, id, nil)
	if err != nil {
		response.ServerError(c, err)
		return
	}
	oldVersion := item.Version
	lockedCompanyID, lockedBranchID := item.CompanyID, item.BranchId
	if err := c.ShouldBindJSON(item); err != nil {
		response.HandleError(c, err)
		return
	}
	item.CompanyID, item.BranchId = lockedCompanyID, lockedBranchID
	item.Version = oldVersion + 1
	res := h.DB.Where("version = ?", oldVersion).Save(item)
	if res.Error != nil {
		response.ServerError(c, res.Error)
		return
	}
	response.Updated(c, item)
}
'''

# Lane multipart: `c.ShouldBind(&item)` — gin lấy TÊN FIELD làm key khi thiếu tag `form`.
FIX_MATERIAL = '''
func (h *Handler) UpdateMaterial(c *gin.Context) {
	var item crmServiceDao.Material
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	if err := tenant.JSONCompanyScope(c, tenant.ScopedDBNullBranch(c, h.DB, "material"), "companyId").First(&item, id).Error; err != nil {
		response.NotFound(c)
		return
	}
	if err := c.ShouldBind(&item); err != nil {
		response.HandleError(c, err)
		return
	}
	if strings.TrimSpace(item.CodeMaterial) == "" {
		item.CodeMaterial = fmt.Sprintf("VT%s%05d", time.Now().Format("0601"), item.ID)
	}
	if err := h.DB.Set("userID", middleware.GetUserID(c)).Save(&item).Error; err != nil {
		response.ServerError(c, err)
		return
	}
	response.Updated(c, item)
}
'''

# --- fixture ÂM: các khuôn AN TOÀN đã xác nhận tay ở story-03/04, KHÔNG được báo đỏ ---
NEG_HELPER = '''
func (h *Handler) UpdateAppointmentType(c *gin.Context) {
	var item dao.AppointmentType
	id, _ := helpers.ParseID(c)
	tenant.ScopedDBNullBranch(c, h.DB, "appointment_type").First(&item, id)
	if err := helpers.BindUpdate(c, &item); err != nil {
		response.HandleError(c, err)
		return
	}
	h.DB.Save(&item)
}
'''
NEG_HELPER_WITH = '''
func (h *Handler) UpdateCompany(c *gin.Context) {
	var item companyDao.Company
	id, _ := helpers.ParseID(c)
	h.DB.First(&item, id)
	if err := helpers.BindUpdateWith(&item, func() error { bindCompanyFormFields(c, &item); return nil }); err != nil {
		response.HandleError(c, err)
		return
	}
	h.DB.Save(&item)
}
'''
NEG_DTO_COPY = '''
func (h *Handler) UpdateUnitDto(c *gin.Context) {
	var item dao.Unit
	var body updateInput
	id, _ := helpers.ParseID(c)
	tenant.ScopedDBNullBranch(c, h.DB, "unit").First(&item, id)
	if err := c.ShouldBindJSON(&body); err != nil {
		response.HandleError(c, err)
		return
	}
	item.Name = body.Name
	item.Note = body.Note
	h.DB.Save(&item)
}
'''
NEG_MAP_MERGE = '''
func (h *Handler) UpdateOption(c *gin.Context) {
	var item dao.UserOption
	body := map[string]interface{}{}
	id, _ := helpers.ParseID(c)
	tenant.ScopedDBNullBranch(c, h.DB, "user_option").First(&item, id)
	if err := c.ShouldBindJSON(&body); err != nil {
		response.HandleError(c, err)
		return
	}
	if v, ok := body["options"]; ok {
		item.Options = v
	}
	h.DB.Save(&item)
}
'''
NEG_ANON_HAS_ID = '''
func (h *Handler) ToggleBranchOpenAppointment(c *gin.Context) {
	var body struct {
		ID uint `json:"id" form:"id"`
	}
	if err := c.ShouldBind(&body); err != nil {
		response.HandleError(c, err)
		return
	}
	var item branchDao.Branch
	if err := tenant.ScopedDBNullBranch(c, h.DB, "branch").First(&item, body.ID).Error; err != nil {
		response.NotFound(c)
		return
	}
	item.OpenAppointment = 1
	h.DB.Save(&item)
}
'''
NEG_VALUE_PIN = '''
func (h *Handler) UpdateServiceTreatmentStep(c *gin.Context) {
	var item dao.ServiceTreatmentStep
	id, _ := helpers.ParseID(c)
	tenant.ScopedDBNullBranch(c, h.DB, "service_treatment_step").First(&item, id)
	if err := c.ShouldBindJSON(&item); err != nil {
		response.HandleError(c, err)
		return
	}
	item.ID = id
	h.DB.Save(&item)
}
'''

# Biến thể B: `body` là struct ẩn danh EMBED entity, seed từ `item` (COPY struct nhưng field con trỏ
# vẫn TRỎ CHUNG ô nhớ với `item`) → bind vào `body` → "khôi phục" `body.F = item.F`. Đã chứng minh
# bằng chương trình Go tối giản: payload `{"created_by":777}` làm CẢ `item.CreatedBy` đổi theo,
# khôi phục xong vẫn ra 777. Nguyên văn từ `med/purchase_order` (bản HIỆN TẠI, chưa vá lớp này).
FIX_ALIAS = '''
func (h *Handler) UpdatePurchaseOrder(c *gin.Context) {
	var item purchaseOrderDao.MedPurchaseOrder
	id, ok := helpers.ParseID(c)
	if !ok {
		response.NotFound(c)
		return
	}
	if err := tenant.ScopedDBNullBranch(c, h.DB, "med_purchase_order").First(&item, id).Error; err != nil {
		response.NotFound(c)
		return
	}
	body := struct {
		purchaseOrderDao.MedPurchaseOrder
		Details []map[string]interface{} `json:"medPurchaseOrderDetail"`
	}{MedPurchaseOrder: item}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.HandleError(c, err)
		return
	}
	body.ID = item.ID
	body.CreatedBy = item.CreatedBy
	body.ApprovedBy = item.ApprovedBy
	if err := h.DB.Save(&body.MedPurchaseOrder).Error; err != nil {
		response.ServerError(c, err)
		return
	}
	response.Updated(c, body)
}
'''

# Copy DTO → entity lúc DỰNG bản ghi (chiều NGƯỢC lại: biến nhận payload là `input`, không phải
# `entry`) — KHÔNG phải pin, không được báo. Nguyên văn khuôn `emr/audit_log`.
NEG_DTO_TO_ENTITY = '''
func (h *Handler) Log(c *gin.Context) {
	var input feLogInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.HandleError(c, err)
		return
	}
	entry := auditDao.EmrAuditLog{
		EntityType: input.EntityType,
		Action:     input.Action,
	}
	entry.MedicalRecordId = input.MedicalRecordId
	h.DB.Create(&entry)
}
'''

SELF_TEST_POS = [
    ('UpdateEmrBed', FIX_EMR_BED, 'ID-UNPINNED', 'config/emr_bed — pin CompanyID/BranchId nhưng KHÔNG pin ID'),
    ('UpdateBranch', FIX_BRANCH, 'ID-UNPINNED', 'config/branch — 2 lane (multipart thủ công + JSON), không pin'),
    ('Update', FIX_TAX_RATE, 'PIN-PTR', 'accountant/tax_rate — ID pin đúng, nhưng pin CompanyID *uint VÔ HIỆU'),
    ('UpdateEmrDepartment', FIX_EMR_DEPARTMENT, 'ID-UNPINNED', 'dạng ShouldBindJSON(item) KHÔNG có `&`'),
    ('UpdateMaterial', FIX_MATERIAL, 'ID-UNPINNED', 'lane multipart c.ShouldBind(&item)'),
    ('UpdatePurchaseOrder', FIX_ALIAS, 'PIN-ALIAS', 'med biến thể B — khôi phục từ biến DÙNG CHUNG con trỏ'),
]
SELF_TEST_NEG = [
    ('helpers.BindUpdate', NEG_HELPER),
    ('helpers.BindUpdateWith + binder thủ công', NEG_HELPER_WITH),
    ('bind DTO rồi copy tường minh', NEG_DTO_COPY),
    ('bind map rồi merge từng key', NEG_MAP_MERGE),
    ('DTO ẩn danh ANON-HAS-ID (toggle)', NEG_ANON_HAS_ID),
    ('pin tay ID kiểu giá trị, vô điều kiện', NEG_VALUE_PIN),
    ('copy DTO → entity lúc dựng bản ghi', NEG_DTO_TO_ENTITY),
]


def scan_src(src):
    """Kết quả gộp FINDING + WARN — self-test soi cả 2 (PIN-ALIAS/PIN-PTR-ARG là WARN)."""
    out = []
    for name, body, line in funcs(src):
        f, w = scan_func(name, body, line)
        out += [(k, name) for k, _ in f + w]
    return out


def self_test():
    ok = True
    for want_func, src, want_kind, why in SELF_TEST_POS:
        got = [k for k, n in scan_src(src) if n == want_func]
        if want_kind in got:
            print(f'  ✓ {want_kind:12s} `{want_func}` — {why}')
        else:
            print(f'  ✗ KHÔNG bắt được {want_kind} ở `{want_func}` (được: {got or "rỗng"}) — {why}')
            ok = False
    for why, src in SELF_TEST_NEG:
        got = scan_src(src)
        if got:
            print(f'  ✗ BÁO NHẦM khuôn an toàn "{why}": {got}')
            ok = False
        else:
            print(f'  ✓ không báo nhầm — {why}')
    if not _STRUCTS:
        print('  ✗ index struct rỗng — không phân giải được kiểu field, luật PIN-PTR chết')
        ok = False
    return ok


def main():
    show_all = '--all' in sys.argv
    show_warn = '--warn' in sys.argv
    print('SELF-TEST (bắt buộc — xem docblock):')
    if not self_test():
        print('\n⛔ Self-test FAIL. Sửa luật rồi chạy lại. KHÔNG dùng kết quả quét.')
        return 2

    findings, warns = [], []
    for p, src in iter_handlers():
        rel = p.replace(BE + '/', '').replace('internal/modules/', '')
        for name, body, line in funcs(src):
            f, w = scan_func(name, body, line)
            for kind, msg in f:
                key = f'{name}@{rel}'
                if key in BASELINE and not show_all:
                    continue
                findings.append((kind, rel, line, msg, BASELINE.get(key, '')))
            for kind, msg in w:
                warns.append((kind, rel, line, msg))

    print(f'\nQuét {MODULES.replace(BE + "/", "")} — {len(findings)} FINDING'
          f'{" (gồm cả BASELINE)" if show_all else " ngoài BASELINE"}, {len(warns)} WARN\n')
    for kind, rel, line, msg, note in sorted(findings):
        tail = f'   [baseline: {note}]' if note else ''
        print(f'  {kind:12s} {rel}:{line}  {msg}{tail}')
    if not findings:
        print('  → sạch (không có điểm nào bind thô vào biến rồi ghi theo id trong struct).')

    if warns:
        print(f'\nWARN — không fail build, nhưng PHẢI đọc tay ({len(warns)} mục'
              f'{"" if show_warn else ", in 5 mục đầu; `--warn` để xem hết"}):')
        for kind, rel, line, msg in (sorted(warns) if show_warn else sorted(warns)[:5]):
            print(f'  {kind:16s} {rel}:{line}  {msg}')

    if findings:
        print('\nMỗi FINDING phải ĐỌC TAY. Cách vá: `helpers.BindUpdate(c, &item[, "Field"...])`'
              ' (pkg/helpers/bind_update.go). Xác nhận an toàn → thêm vào BASELINE kèm lý do.')
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
