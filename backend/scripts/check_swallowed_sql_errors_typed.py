#!/usr/bin/env python3
"""check_swallowed_sql_errors_typed.py — quét `.Raw(...).Scan(&x)` với `x` KHÔNG phải map, không check
`.Error`.

Epic `raw-scan-aggregate-error-sweep` (2026-07-31, story-01) — Phase 2 của epic
`swallowed-sql-errors-sweep` (đã Done). Epic cũ + script
`check_swallowed_sql_errors.py` CHỈ quét đích `map[string]interface{}`/`map[string]any` (quyết định cố ý
ở story-01 của epic cũ). Script này KHÔNG sửa lại script cũ — tách file riêng, tiêu chí phát hiện đơn
giản hơn: "có .Raw().Scan(), thiếu error check" là đủ điều kiện báo, bất kể kiểu đích (decimal.Decimal,
int64, struct aggregate...).

Rule tương ứng: skill/backend/rules/backend.md §"Lỗi SQL bị nuốt → 200 + list rỗng".

Cách bắt: dòng có `.Raw(` (không bắt đầu bằng `if err := `/`if err = ` — form đó đã tự handle tại chỗ
gán), theo sau trong ≤10 dòng có `.Scan(&varname` (varname bất kỳ), và trong 5 dòng kể từ dòng có
`.Scan(` KHÔNG có `.Error` hoặc token `err`.

Đây là heuristic — CÓ false-positive (comment, biến `err` xuất hiện vì lý do khác không liên quan) và
false-negative (assignment gián tiếp, biến đặt tên lạ, error check ở xa hơn 5 dòng). Không thay thế
review tay — epic này BẮT BUỘC audit tay từng điểm trước khi vá (xem story-02/03).

Usage:
    python3 scripts/check_swallowed_sql_errors_typed.py [path...]   # mặc định: internal/modules/webapp
    python3 scripts/check_swallowed_sql_errors_typed.py --ci [path...]

Exit code: 0 = sạch (hoặc mode mặc định, chỉ báo cáo). 1 = có phát hiện VÀ chạy với --ci.
"""
import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
DEFAULT_ROOTS = ["internal/modules/webapp"]

RAW_RE = re.compile(r"\.Raw\(")
RAW_HANDLED_PREFIX_RE = re.compile(r"^\s*(?:\w+\s*,\s*)?err\s*:?=\s*.*\.Raw\(|^\s*if\s+err\s*:?=\s*.*\.Raw\(")
SCAN_RE = re.compile(r"\.Scan\(&(\w+)")
MAP_DECL_RE = re.compile(r"\b(\w+)\s*(?::?=|\s)\s*(?:\[\]?)?map\[string\](?:interface\{\}|any)")
MAX_LOOKAHEAD_LINES = 25
ERROR_WINDOW_LINES = 5


def scan_file(path):
    try:
        with open(path, encoding="utf-8", errors="ignore") as fh:
            lines = fh.readlines()
    except OSError:
        return []

    # Biến khai báo kiểu map trong file — dùng để loại trừ (thuộc script cũ, tránh double-report).
    map_vars = set()
    for line in lines:
        m = MAP_DECL_RE.search(line)
        if m:
            map_vars.add(m.group(1))

    findings = []
    for i, line in enumerate(lines):
        if line.strip().startswith("//"):
            continue
        if not RAW_RE.search(line):
            continue
        if RAW_HANDLED_PREFIX_RE.search(line):
            continue

        scan_idx = None
        scan_var = None
        for j in range(i, min(i + MAX_LOOKAHEAD_LINES, len(lines))):
            m = SCAN_RE.search(lines[j])
            if m:
                scan_idx = j
                scan_var = m.group(1)
                break
        if scan_idx is None:
            continue
        if scan_var in map_vars:
            continue  # đích map — thuộc script cũ

        window = "".join(lines[scan_idx : scan_idx + ERROR_WINDOW_LINES])
        if ".Error" in window:
            continue
        if re.search(r"\berr\b", window):
            continue

        findings.append(f"{path}:{i+1}: {line.strip()}")
    return findings


def main():
    ci_mode = "--ci" in sys.argv
    roots = [a for a in sys.argv[1:] if a != "--ci"]
    if not roots:
        roots = DEFAULT_ROOTS

    all_findings = []
    for root in roots:
        full_root = os.path.join(ROOT, root) if not os.path.isabs(root) else root
        if os.path.isfile(full_root):
            rel = os.path.relpath(full_root, ROOT)
            all_findings.extend(scan_file(full_root))
            continue
        for dirpath, _, filenames in os.walk(full_root):
            for fn in filenames:
                if fn.endswith(".go") and not fn.endswith("_test.go"):
                    all_findings.extend(scan_file(os.path.join(dirpath, fn)))

    if not all_findings:
        print("check_swallowed_sql_errors_typed: 0 điểm nghi ngờ — sạch.")
        return 0

    print(f"check_swallowed_sql_errors_typed: {len(all_findings)} điểm nghi ngờ (Raw().Scan() đích TYPED không kiểm .Error):\n")
    for f in all_findings:
        print(f"  {f}")
    print(
        "\nMỗi điểm: audit tay trước khi vá (KHÔNG tin thẳng script). Nếu xác nhận thật, thêm "
        "`tx := ...Raw(...).Scan(&x)` + `if pagination.IsQueryError(tx.Error) { "
        "pagination.FailQuery(c, tx.Error); return }` — xem skill/backend/rules/backend.md."
    )
    if ci_mode:
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
