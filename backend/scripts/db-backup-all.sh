#!/bin/bash
# Backup các DB được CHỈ ĐỊNH RÕ (không còn quét pattern cluvix_% — pattern cũ bắt cả DB ERP).
# Dump từng DB riêng + verify.
#
#   ./db-backup-all.sh --db cluvix_crm [--db <tên-khác> ...] [--out <thư-mục>]
#
# Cấu hình:
#   ALLOWED_DBS   danh sách tên DB được phép thao tác (cách nhau bởi dấu cách/phẩy).
#                 Mặc định: "cluvix_crm". DB ngoài danh sách => từ chối. `cluvix_drc` (ERP) luôn bị chặn.
#   MYSQL_USER, MYSQL_PWD   tài khoản MySQL (không có thì hỏi). Mật khẩu KHÔNG đi qua tham số
#                 dòng lệnh (không lộ trong `ps`) — ghi vào file --defaults-extra-file tạm chmod 600,
#                 xoá khi thoát.
#   MYSQL_HOST, MYSQL_PORT  (tuỳ chọn)
#
# Exit != 0 nếu BẤT KỲ DB nào fail gate. Chưa PASS hết thì KHÔNG được coi là có backup.

set -uo pipefail

ALLOWED="${ALLOWED_DBS:-cluvix_crm}"
ALLOWED="${ALLOWED//,/ }"
NEVER="cluvix_drc"
OUT=""
TARGETS=()

while [ $# -gt 0 ]; do
  case "$1" in
    --db)  TARGETS+=("${2:?--db cần tên DB}"); shift 2 ;;
    --out) OUT="${2:?--out cần thư mục}"; shift 2 ;;
    *) echo "❌ Tham số lạ: $1"; exit 2 ;;
  esac
done
[ ${#TARGETS[@]} -gt 0 ] || { echo "❌ Phải chỉ định ít nhất một --db <tên>"; exit 2; }
OUT="${OUT:-$HOME/sql-backup-$(date +%Y%m%d-%H%M%S)}"

in_list() { local x; for x in $2; do [ "$x" = "$1" ] && return 0; done; return 1; }

for db in "${TARGETS[@]}"; do
  [[ "$db" =~ ^[A-Za-z0-9_]+$ ]] || { echo "❌ Tên DB không hợp lệ: $db"; exit 2; }
  in_list "$db" "$NEVER"   && { echo "❌ $db bị chặn vĩnh viễn (DB ERP)"; exit 2; }
  in_list "$db" "$ALLOWED" || { echo "❌ $db không nằm trong ALLOWED_DBS ($ALLOWED)"; exit 2; }
done

MU="${MYSQL_USER:-}"
[ -n "$MU" ] || read -rp "MySQL user: " MU
MP="${MYSQL_PWD:-}"
if [ -z "$MP" ]; then read -rsp "MySQL password: " MP; echo; fi
unset MYSQL_PWD

CNF=$(mktemp) || exit 1
chmod 600 "$CNF"
trap 'rm -f "$CNF"' EXIT
{
  echo "[client]"
  echo "user=$MU"
  echo "password=\"${MP//\\/\\\\}\""
  [ -n "${MYSQL_HOST:-}" ] && echo "host=$MYSQL_HOST"
  [ -n "${MYSQL_PORT:-}" ] && echo "port=$MYSQL_PORT"
} > "$CNF"
unset MP

M="mysql --defaults-extra-file=$CNF"
DUMP="mysqldump --defaults-extra-file=$CNF"

mkdir -p "$OUT" || exit 1

echo "Sẽ dump ${#TARGETS[@]} DB → $OUT"
FAILED=0

for db in "${TARGETS[@]}"; do
  f="$OUT/$db.sql"
  echo "=== $db"

  # --routines --events BẮT BUỘC: thiếu thì procedure/event mất im lặng, file vẫn hợp lệ.
  # --single-transaction: không khoá bảng, prod vẫn ghi được trong lúc dump.
  $DUMP --single-transaction --quick \
        --routines --events \
        --default-character-set=utf8mb4 \
        --set-gtid-purged=OFF --hex-blob \
        "$db" > "$f" 2> "$OUT/$db.err"
  rc=$?

  if [ $rc -ne 0 ]; then
    echo "  ❌ mysqldump exit $rc — xem $OUT/$db.err"; FAILED=1; continue
  fi

  # GATE 1 — file kết thúc trọn vẹn (dump đứt giữa chừng vẫn exit 0)
  if ! tail -1 "$f" | grep -q "^-- Dump completed on"; then
    echo "  ❌ THIẾU '-- Dump completed on' — DUMP ĐỨT GIỮA CHỪNG"; FAILED=1; continue
  fi

  # GATE 2 — số bảng trong dump khớp DB thật
  want=$($M -N -e "SELECT COUNT(*) FROM information_schema.tables
                   WHERE table_schema='$db' AND table_type='BASE TABLE';" 2>/dev/null)
  got=$(grep -c "^CREATE TABLE" "$f")
  if [ "$want" != "$got" ]; then
    echo "  ❌ bảng: DB có $want, dump có $got"; FAILED=1; continue
  fi

  # GATE 3 — cảnh báo stderr (thiếu quyền, view hỏng...)
  if grep -v "Warning.*password" "$OUT/$db.err" 2>/dev/null | grep -q .; then
    echo "  ⚠  stderr:"; grep -v "Warning.*password" "$OUT/$db.err"
  fi

  echo "  ✅ $want bảng, $(du -h "$f" | cut -f1)"
done

echo
if [ $FAILED -ne 0 ]; then
  echo "❌❌ CÓ DB DUMP LỖI — KHÔNG ĐƯỢC GỠ MYSQL / COI ĐÂY LÀ BACKUP"
  exit 1
fi

( cd "$OUT" && { command -v sha256sum >/dev/null && sha256sum ./*.sql || shasum -a 256 ./*.sql; } > SHA256SUMS )
echo "✅ Tất cả PASS → $OUT"
echo "⚠  Chưa xong: copy thư mục này RA KHỎI MÁY, rồi restore-test bằng db-restore-all.sh --test"
