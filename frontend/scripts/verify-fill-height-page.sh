#!/usr/bin/env bash
# Gate cơ học cho epic `p-table-full-height-list-pages` (bmad/p-table-full-height-list-pages/).
#
# `appFillHeight="page"` là attribute TĨNH trên <p-table> — thiếu `FillHeightDirective` trong
# mảng `imports: [...]` của component KHÔNG bị `tsc`/`ngc`/`ng build` bắt lỗi nào (đã đo thực
# nghiệm, xem bmad/p-table-full-height-list-pages/pilot-findings.md §"Gate cơ học"). Angular chỉ
# âm thầm bỏ qua attribute lạ — bảng mất ràng buộc chiều cao, tự cao theo nội dung, dễ lọt qua
# smoke test bằng mắt với list ít dòng.
#
# Dùng: chạy từ thư mục `frontend/`.
#   ./scripts/verify-fill-height-page.sh
# Output rỗng + exit 0 = PASS. Có dòng "THIEU IMPORT: ..." + exit 1 = FAIL.

set -uo pipefail
cd "$(dirname "$0")/.." || exit 1

fail=0
while IFS= read -r html; do
  ts="${html%.html}.ts"
  if [ ! -f "$ts" ]; then
    echo "KHONG CO .ts TUONG UNG: $html"
    fail=1
    continue
  fi
  if ! grep -q "FillHeightDirective" "$ts"; then
    echo "THIEU IMPORT: $html"
    fail=1
  fi
done < <(grep -rl "appFillHeight" src/app --include="*.html")

if [ "$fail" -eq 0 ]; then
  echo "GATE PASS"
fi
exit "$fail"
