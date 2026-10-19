#!/usr/bin/env python3
"""idor-scan.py — quét handler nhận id TỪ CLIENT rồi đọc/ghi DB mà KHÔNG scope, KHÔNG guard chủ sở hữu.

Sinh ra sau khi vá lỗ IDOR `GET /config/user-profile/:id` (commit `eb9e6c1d`, 2026-08-12): bất kỳ user
đã đăng nhập nào cũng đọc được hồ sơ người khác. Script giữ vai trò LƯỚI CHỐNG TÁI SINH cho code MỚI —
chạy tay trước khi merge PR đụng `internal/modules/**/handler/**` (chưa gắn CI, cùng lý do với
`check_swallowed_sql_errors.py`).

Rule tương ứng: `skill/backend/rules/backend.md` §Tenant scope + CLAUDE.md Hard Rules
("GET/UPDATE/DELETE theo id phải qua `tenant.ScopedDBNullBranch`").

⚠ ĐỌC TRƯỚC KHI TIN KẾT QUẢ — vì sao có `SELF_TEST`:
    Bản đầu của script này coi `middleware.GetUserID(c)` là "dấu hiệu đã bảo vệ". Nhưng ở bản CÓ LỖ,
    `GetUserID` chỉ dùng làm FALLBACK khi `?id=` rỗng — tức chính là mẫu tấn công. Luật đó triệt tiêu
    đúng lớp true-positive cần tìm: script báo "sạch" trong khi bug thật nằm ngay trong phạm vi quét.
    Vì vậy script LUÔN tự kiểm trên bản code TRƯỚC commit vá; self-test fail ⇒ exit 2, KHÔNG in kết quả.
    Thêm/bớt token vào OWNER_GUARD phải chạy lại self-test.

Ngưỡng phán đoán "đã bảo vệ" = có GUARD thật, KHÔNG phải "có gọi hàm lấy identity":
    - scope tenant (`tenant.Scoped*`, helper `*Scope*(c)`, `ScopedClient`, `BuildUserQuery`…), HOẶC
    - ràng buộc chủ sở hữu (`WHERE user_id = <từ JWT>`, `verify*Ownership`, `IsCompanyAllowed`,
      `GetCompanyIDs`, `Forbidden(`…), HOẶC
    - uỷ quyền cho service/helper đã scope (`h.XxxSvc.`, `DeleteWithCheck`, `loadRequestForAction`…).

Exit code: 0 = sạch · 1 = có finding mới (ngoài BASELINE) · 2 = self-test fail (script hỏng).

Dùng: python3 scripts/idor-scan.py [--all]   (--all: in cả mục đã nằm trong BASELINE)
"""
import os
import re
import subprocess
import sys

BE = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
ROOT = os.path.join(BE, 'internal/modules')

FUNC_RE = re.compile(r'^func \(h \*Handler\) (\w+)\(c \*gin\.Context\) \{', re.M)

# --- id đến TỪ CLIENT -------------------------------------------------------
ID_FROM_URL = re.compile(r'c\.Param\("(id|user_id|userId)"\)|c\.Query\("(id|user_id|userId)"\)|helpers\.ParseID\(c\)')
ID_FROM_BODY = re.compile(r'ShouldBind\w*\(|c\.GetPostForm\(')

# --- chạm DB ----------------------------------------------------------------
TOUCH_DB = re.compile(r'h\.DB|\.Table\(|\.Model\(|\.First\(|\.Find\(|\.Updates\(|\.Delete\(|\.Save\(')
MUTATING = re.compile(r'\.Updates\(|\.Delete\(|\.Save\(|\.Create\(|\.Exec\(')

# --- GUARD thật (xem docblock: KHÔNG liệt `GetUserID(c)` đơn thuần) ----------
OWNER_GUARD = re.compile(
    r'tenant\.Scoped\w*\('
    r'|ScopedClient|scopedDB|ScopedDB|BuildUserQuery|leadscope\.Apply'
    r'|[Ss]cope\w*\(c|\w+Scope\(|InScope\('
    r'|verify\w*Ownership|IsCompanyAllowed|companyAllowed|branchAllowed'
    r'|GetCompanyActiveID|GetCompanyIDs|companyIDs|GetPartnerCompanyID|GetPartner'
    r'|LookupCustomerBy|DeleteWithCheck|loadRequestForAction|loadMyRequest'
    r'|resolveEmployeeCode'
    r'|Forbidden\('
    r'|h\.\w*(Svc|Service)\.'
    r'|repository\.New\[|repo\.\w+\(c'   # BaseRepo.scope(c) — có test out-of-scope
)

# --- Bảng có RANH GIỚI TENANT hay không -------------------------------------
# Nguồn chân lý = registry `tenantTables`/`branchTables` trong pkg/tenant/scope.go, KHÔNG đoán từ
# struct dao. Bài học 2026-08-12: bản trước suy từ "struct có field BranchId" → gắn cờ nhầm
# warehouse/med_medicine_config (cả 2 KHÔNG nằm trong registry nào ⇒ `ScopedDBNullBranch` là NO-OP,
# thêm vào chỉ tốn diff mà không đổi hành vi). Branch KHÔNG phải trục tenant — trục tenant là company.
_TENANT, _BRANCH, _ENT2TBL = set(), set(), {}


def _load_registry():
    if _TENANT:
        return
    import glob
    src = open(os.path.join(BE, 'pkg/tenant/scope.go'), encoding='utf-8').read()
    for var, dest in (('tenantTables', _TENANT), ('branchTables', _BRANCH)):
        m = re.search(r'var ' + var + r' = map\[string\]bool\{(.*?)\n\}', src, re.S)
        if m:
            dest.update(re.findall(r'"(\w+)":\s*true', m.group(1)))
    # entity -> tên bảng (TableName())
    for p in glob.glob(os.path.join(BE, 'internal/**/dao/*.go'), recursive=True):
        t = open(p, encoding='utf-8', errors='replace').read()
        for m in re.finditer(r'func \((\w+)\) TableName\(\) string \{ return "(\w+)" \}', t):
            _ENT2TBL[m.group(1)] = m.group(2)


ENT_REF = re.compile(r'(?:\w*[Dd]ao|models)\.(\w+)\b')
TBL_REF = re.compile(r'\.Table\("(\w+)"')


def target_in_tenant_scope(body):
    """True = bảng đích CÓ ranh giới tenant/branch thật (đáng soi) · False = không · None = chưa rõ."""
    _load_registry()
    tbls = set(TBL_REF.findall(body))
    tbls |= {_ENT2TBL[e] for e in ENT_REF.findall(body) if e in _ENT2TBL}
    if not tbls:
        return None
    return any((t in _TENANT) or (t in _BRANCH) for t in tbls)


# --- Baseline: đã đọc tay 2026-08-12, xác nhận CÓ bảo vệ hoặc không có ranh giới tenant ---
# Khoá = "<func>@<đuôi path>". Sửa/xoá entry chỉ khi đã đọc lại code, KHÔNG xoá cho hết đỏ.
BASELINE = {
    # bảng KHÔNG có company_id/branch_id ⇒ không có ranh giới tenant để vi phạm (1 DB = 1 khách hàng)
    'GetNewPost@zalo/news/handler/handler.go': 'new_post không có cột scope',
    'DeleteNewPost@zalo/news/handler/handler.go': 'new_post không có cột scope',
    'DeleteNewPromotion@zalo/news/handler/handler.go': 'new_promotion không có cột scope',
    'DeleteNewSlide@zalo/news/handler/handler.go': 'new_slide không có cột scope',
    'DeleteNewVideo@zalo/news/handler/handler.go': 'new_video không có cột scope',
    'DeleteZaloGameCampaign@zalo/game/handler/handler.go': 'bảng game không có cột scope',
    'DeleteZaloGameGift@zalo/game/handler/handler.go': 'bảng game không có cột scope',
    'ActiveSmsOption@config/sms_option/handler/handler.go': 'sms_option không có cột scope (config DB-wide)',
    # nội dung DB-wide cho app khách; nhánh ?id= đi qua View* có ScopedClient + customer_id
    'ListNewPost@client/customer/handler/news.go': 'ScopedClient + LookupCustomerByZaloID ở nhánh view',
    'ListNewPromotion@client/customer/handler/news.go': 'như trên',
    'ListNewSlide@client/customer/handler/news.go': 'như trên',
    'ListNewVideo@client/customer/handler/news.go': 'như trên',
}


def iter_handlers():
    for dp, _, fs in os.walk(ROOT):
        for fn in fs:
            if not fn.endswith('.go') or fn.endswith('_test.go'):
                continue
            p = os.path.join(dp, fn)
            if '/handler/' not in p and not fn.startswith('handler'):
                continue
            src = open(p, encoding='utf-8', errors='replace').read()
            yield p, src


def funcs(src):
    ms = list(FUNC_RE.finditer(src))
    for k, m in enumerate(ms):
        end = ms[k + 1].start() if k + 1 < len(ms) else len(src)
        yield m.group(1), src[m.start():end], src[:m.start()].count('\n') + 1


def is_suspect(body, id_re):
    return bool(id_re.search(body)) and bool(TOUCH_DB.search(body)) and not OWNER_GUARD.search(body)


# ---------------------------------------------------------------- self-test
SELF_TEST = [
    # (commit vá, path trong repo, tên handler PHẢI bị bắt ở bản TRƯỚC vá)
    ('eb9e6c1d', 'internal/modules/webapp/config/user_profile/handler/handler.go', 'GetUserProfile'),
]


def self_test():
    ok = True
    for commit, path, want in SELF_TEST:
        try:
            src = subprocess.check_output(
                ['git', 'show', f'{commit}^:./{path}'], cwd=BE, text=True, stderr=subprocess.DEVNULL)
        except subprocess.CalledProcessError:
            print(f'  ✗ không lấy được {commit}^:{path} (commit đã bị rewrite?)')
            ok = False
            continue
        hit = [n for n, b, _ in funcs(src) if is_suspect(b, ID_FROM_URL)]
        if want in hit:
            print(f'  ✓ bắt được `{want}` ở {commit}^ (bản trước vá)')
        else:
            print(f'  ✗ KHÔNG bắt được `{want}` ở {commit}^ — luật đã hỏng, kết quả quét vô nghĩa')
            ok = False
    return ok


def main():
    show_all = '--all' in sys.argv
    print('SELF-TEST (bắt buộc — xem docblock):')
    if not self_test():
        print('\n⛔ Self-test FAIL. Sửa OWNER_GUARD/luật rồi chạy lại. KHÔNG dùng kết quả quét.')
        return 2

    findings = []
    for p, src in iter_handlers():
        rel = p.replace(BE + '/', '').replace('internal/modules/', '')
        for name, body, line in funcs(src):
            axis = None
            if is_suspect(body, ID_FROM_URL):
                axis = 'URL'
            elif ID_FROM_BODY.search(body) and is_suspect(body, ID_FROM_BODY):
                axis = 'BODY'
            if not axis:
                continue
            key = f'{name}@{rel}'
            if key in BASELINE and not show_all:
                continue
            scope = target_in_tenant_scope(body)
            if scope is False and not show_all:
                continue          # bảng không có cột scope ⇒ không có ranh giới tenant để vi phạm
            findings.append((axis, 'MUT' if MUTATING.search(body) else 'GET', name, rel, line,
                             BASELINE.get(key, '') or ('' if scope else 'bảng đích chưa phân giải được')))

    print(f'\nQuét {ROOT.replace(BE + "/", "")} — {len(findings)} điểm cần xem'
          f'{" (gồm cả BASELINE)" if show_all else " ngoài BASELINE"}\n')
    for axis, kind, name, rel, line, note in sorted(findings):
        tail = f'   [baseline: {note}]' if note else ''
        print(f'  {axis:4s} {kind} {name:34s} {rel}:{line}{tail}')

    if not findings:
        print('  → sạch.')
        return 0
    print('\nMỗi điểm phải ĐỌC TAY trước khi kết luận. Xác nhận an toàn → thêm vào BASELINE kèm lý do.')
    return 1


if __name__ == '__main__':
    sys.exit(main())
