#!/usr/bin/env python3
"""check_swallowed_sql_errors.py — quét handler map-scan viết tay không kiểm `.Error`.

Epic swallowed-sql-errors-sweep (2026-07-30, story-06 FR7): sau khi vá xong 194+11 điểm
`.Find(&m)`/`.Scan(&m)`/`.Take(&m)` vào `map[string]interface{}`/`map[string]any` không đọc
`.Error`, script này giữ vai trò LƯỚI CHỐNG TÁI SINH cho code MỚI — chạy tay trước khi merge PR
đụng `internal/modules/webapp/**`/`mobile/v1/**`/`client/quiz/**` (chưa gắn CI thật, xem
skill/bmad/swallowed-sql-errors-sweep/deferred-work.md — quyết định CI hạ tầng chung chưa chốt,
tránh mở rộng epic này sang việc dựng CI).

Rule tương ứng: skill/backend/rules/backend.md §"Lỗi SQL bị nuốt → 200 + list rỗng".

Cách bắt: khai báo biến kiểu `map[string]interface{}`/`map[string]any` (hoặc slice của nó) trong
CÙNG SCOPE gần (≤40 dòng) với lời gọi `.Find`/`.Scan`/`.Take` trên nó, KHÔNG đi qua
`pagination.Paginate`/`PaginateWithTotal`, và không có `.Error`/token `err` trong 5 dòng sau lời gọi.

Đây là heuristic — CÓ false-positive (biến trùng tên khác kiểu trong cùng file, hiếm) và
false-negative (assignment gián tiếp, biến đặt tên lạ). Không thay thế review tay, chỉ giảm khả
năng bỏ sót rõ ràng.

Usage:
    python3 scripts/check_swallowed_sql_errors.py            # human-readable, in ra danh sách
    python3 scripts/check_swallowed_sql_errors.py --ci       # exit 1 nếu có phát hiện (dùng cho CI/pre-commit khi được wire)

Exit code: 0 = sạch (hoặc mode mặc định, chỉ báo cáo). 1 = có phát hiện VÀ chạy với --ci.
"""
import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
ROOTS = [
    "internal/modules/webapp",
    "internal/modules/mobile/v1",
    "internal/modules/client/quiz",
]

CALL_RE = re.compile(r'(?:^\s*|\.)(Find|Scan|Take)\(&(\w+)')
DECL_RE = re.compile(r'\b(\w+)\s*(?::?=|\s)\s*(?:\[\]?)?map\[string\](?:interface\{\}|any)')
VAR_DECL_RE = re.compile(r'var\s+(\w+)\s+(?:\[\])?map\[string\](?:interface\{\}|any)')
MAX_SCOPE_LINES = 40


def scan_file(path):
    try:
        with open(path, encoding="utf-8", errors="ignore") as fh:
            lines = fh.readlines()
    except OSError:
        return []

    map_vars = set()
    decl_line = {}
    for i, line in enumerate(lines):
        m = VAR_DECL_RE.search(line)
        if m:
            map_vars.add(m.group(1))
            decl_line[m.group(1)] = i
        m2 = DECL_RE.search(line)
        if m2:
            map_vars.add(m2.group(1))
            decl_line[m2.group(1)] = i

    findings = []
    for i, line in enumerate(lines):
        if "Paginate" in line:
            continue
        if line.strip().startswith("//"):
            continue
        m = CALL_RE.search(line)
        if not m:
            continue
        varname = m.group(2)
        if varname not in map_vars:
            continue
        di = decl_line.get(varname, -1)
        if di < 0 or i - di > MAX_SCOPE_LINES or di > i:
            continue
        window = "".join(lines[i : i + 5])
        if ".Error" in window:
            continue
        if re.search(r"\berr\b", window):
            continue
        findings.append(f"{path}:{i+1}: {line.strip()}")
    return findings


def main():
    ci_mode = "--ci" in sys.argv
    all_findings = []
    for root in ROOTS:
        full_root = os.path.join(ROOT, root)
        for dirpath, _, filenames in os.walk(full_root):
            for fn in filenames:
                if fn.endswith(".go"):
                    rel = os.path.relpath(os.path.join(dirpath, fn), ROOT)
                    all_findings.extend(scan_file(os.path.join(dirpath, fn)))

    if not all_findings:
        print("check_swallowed_sql_errors: 0 điểm nghi ngờ — sạch.")
        return 0

    print(f"check_swallowed_sql_errors: {len(all_findings)} điểm nghi ngờ (map-scan không kiểm .Error):\n")
    for f in all_findings:
        print(f"  {f}")
    print(
        "\nMỗi điểm: nếu là handler map-scan viết tay thật (không phải false-positive tên biến "
        "trùng), thêm `tx := ...Find(&x)` + `if pagination.IsQueryError(tx.Error) { "
        "pagination.FailQuery(c, tx.Error); return }` — xem skill/backend/rules/backend.md."
    )
    if ci_mode:
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
