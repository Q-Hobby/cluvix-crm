#!/usr/bin/env bash
# check_validation.sh — scan handler Create/Update funcs không có validation marker.
#
# Marker hợp lệ (cần ít nhất 1 trong body):
#   - response.RequireName / response.RequireFields
#   - requireName / requireFields                  (config package)
#   - validation.New(
#   - validate<Entity>(                            (helper riêng từ Phase 1)
#   - binding:"required"                           (gin struct tag)
#
# Usage:
#   ./scripts/check_validation.sh              # human-readable
#   ./scripts/check_validation.sh --json       # JSON cho CI
# Exit code 0 = pass, 1 = có handler thiếu validation.

set -euo pipefail
ROOT="${ROOT:-$(cd "$(dirname "$0")/.." && pwd)}"
HANDLERS="$ROOT/internal/handlers"

mode="${1:-text}"

# Skip-list: handler có tên Create/Update nhưng không phải CRUD entity (vd CreateLog, UpdateSort).
SKIP_RE='^(CreateLog|UpdateSort|UpdateOrderField|UpdateField|Toggle|UpdateStatus|UpdatePosition)$'

check_func() {
  python3 - "$@" <<'PY'
import re, sys

VALIDATION_RE = re.compile(
    r'(?:\bresponse\.RequireName|\bresponse\.RequireFields|\brequireName|\brequireFields'
    r'|\bvalidation\.New|\bvalidate[A-Z][A-Za-z]+\(|binding:"required"'
    # Handler delegate qua service (Phase 1 đặt validate trong service):
    r'|\b[A-Z][A-Za-z]*Svc\.(?:Create|Update|Validate)[A-Z]?[A-Za-z]*\('
    r'|\b[A-Z][A-Za-z]*Service\.(?:Create|Update|Validate)[A-Z]?[A-Za-z]*\('
    # Local alias `svc := h.XxxSvc` rồi `svc.Create(...)`:
    r'|\bsvc\.(?:Create|Update|Validate)[A-Z]?[A-Za-z]*\('
    # Inline check pattern: `if item.Field == "" { response.Validate(...)` — weak nhưng OK
    r'|response\.Validate\(c,\s*"[^"]*(?:bắt buộc|không được|là bắt buộc|không hợp lệ|không tồn tại|đã tồn tại|phải)'
    # Client-package wrapper:
    r'|clientValidate\(c,\s*"[^"]*(?:bắt buộc|không được|là bắt buộc|không hợp lệ|không tồn tại|thiếu))'
)
FUNC_RE = re.compile(r'^func \([^)]*\) ((?:Create|Update)[A-Z][A-Za-z]+)\(c \*gin\.Context\)')
SKIP_RE = re.compile(sys.argv[1])

missing = []
for path in sys.argv[2:]:
    with open(path) as f:
        lines = f.readlines()
    i = 0
    while i < len(lines):
        m = FUNC_RE.match(lines[i].rstrip())
        if not m:
            i += 1
            continue
        name = m.group(1)
        if SKIP_RE.match(name):
            i += 1
            continue
        # Track brace depth from this line forward
        depth = 0
        body = []
        j = i
        started = False
        while j < len(lines):
            line = lines[j]
            body.append(line)
            for ch in line:
                if ch == '{':
                    depth += 1
                    started = True
                elif ch == '}':
                    depth -= 1
            j += 1
            if started and depth == 0:
                break
        block = ''.join(body)
        if not VALIDATION_RE.search(block):
            missing.append((path, name))
        i = j
for p, n in missing:
    print(f"{p}\t{n}")
PY
}

files=()
while IFS= read -r f; do files+=("$f"); done < <(find "$HANDLERS" -name "*.go" -not -name "*_test.go" -type f)

results=$(check_func "$SKIP_RE" "${files[@]}")

count=0
if [ -n "$results" ]; then
  count=$(echo "$results" | wc -l | tr -d ' ')
fi

if [ "$mode" = "--json" ]; then
  printf '{"missing_count":%d,"items":[' "$count"
  if [ "$count" -gt 0 ]; then
    first=1
    while IFS=$'\t' read -r file fn; do
      [ -z "$file" ] && continue
      [ "$first" -eq 1 ] || printf ','
      first=0
      printf '{"file":"%s","func":"%s"}' "$file" "$fn"
    done <<< "$results"
  fi
  printf ']}\n'
else
  if [ "$count" -eq 0 ]; then
    echo "✅ Validation OK — tất cả Create/Update handler có validation marker."
    exit 0
  fi
  echo "❌ $count handler thiếu validation:"
  while IFS=$'\t' read -r file fn; do
    [ -z "$file" ] && continue
    rel="${file#$ROOT/}"
    echo "  $rel  →  $fn"
  done <<< "$results"
  echo ""
  echo "Fix: thêm 1 trong các marker (response.RequireFields, validation.New, validate<Entity>...)."
  echo "Tham khảo: backend/docs/validation-registry.md"
fi

[ "$count" -eq 0 ] || exit 1
