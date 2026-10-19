#!/bin/bash
# Import lại dump do db-backup-all.sh tạo — CHỈ vào DB được chỉ định rõ, có xác nhận.
#
#   ./db-restore-all.sh <thư-mục-dump> --db cluvix_crm [--db ...] [--yes] [--test]
#
#   --db NAME  DB đích (lặp được). Dump tương ứng phải là <thư-mục-dump>/NAME.sql.
#   --yes      bỏ bước gõ lại tên DB (CHỈ cho tự động hoá có chủ đích).
#   --test     import vào DB tạm `restoretest_<tên>` rồi DROP ngay sau verify (chỉ drop DB do script
#              tự tạo; từ chối nếu DB tạm đã tồn tại). Chứng minh dump restore được.
#              Không có cờ này = import THẬT, GHI ĐÈ DB đích.
#
# Cấu hình: ALLOWED_DBS (mặc định "cluvix_crm"; DB ngoài danh sách => từ chối; `cluvix_drc` luôn
# bị chặn), MYSQL_USER/MYSQL_PWD (không có thì hỏi; mật khẩu đi qua file --defaults-extra-file tạm
# chmod 600, xoá khi thoát — KHÔNG qua -p), MYSQL_HOST/MYSQL_PORT.
#
# Chạy bằng user khớp DEFINER trong dump (kiểm: grep -o "DEFINER=[^ ]*" *.sql | sort -u),
# KHÔNG chạy bằng root — view sẽ mang definer là user không tồn tại.

set -uo pipefail

ALLOWED="${ALLOWED_DBS:-cluvix_crm}"
ALLOWED="${ALLOWED//,/ }"
NEVER="cluvix_drc"
SRC=""; YES=0; TEST=0; TARGETS=()

while [ $# -gt 0 ]; do
  case "$1" in
    --db)   TARGETS+=("${2:?--db cần tên DB}"); shift 2 ;;
    --yes)  YES=1; shift ;;
    --test) TEST=1; shift ;;
    -*)     echo "❌ Tham số lạ: $1"; exit 2 ;;
    *)      [ -z "$SRC" ] && SRC="$1" || { echo "❌ Thừa tham số: $1"; exit 2; }; shift ;;
  esac
done
[ -n "$SRC" ] || { echo "❌ Thiếu thư mục chứa file .sql"; exit 2; }
[ -d "$SRC" ] || { echo "❌ Không thấy thư mục $SRC"; exit 1; }
[ ${#TARGETS[@]} -gt 0 ] || { echo "❌ Phải chỉ định ít nhất một --db <tên>"; exit 2; }

in_list() { local x; for x in $2; do [ "$x" = "$1" ] && return 0; done; return 1; }

for db in "${TARGETS[@]}"; do
  [[ "$db" =~ ^[A-Za-z0-9_]+$ ]] || { echo "❌ Tên DB không hợp lệ: $db"; exit 2; }
  in_list "$db" "$NEVER"   && { echo "❌ $db bị chặn vĩnh viễn (DB ERP)"; exit 2; }
  in_list "$db" "$ALLOWED" || { echo "❌ $db không nằm trong ALLOWED_DBS ($ALLOWED)"; exit 2; }
  [ -f "$SRC/$db.sql" ]    || { echo "❌ Không có file $SRC/$db.sql"; exit 2; }
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

$M -e "SELECT 1;" >/dev/null 2>&1 || { echo "❌ Không kết nối được MySQL"; exit 1; }

# .env dùng DB_HOST=127.0.0.1 thì user phải là 'x'@'127.0.0.1' (TCP),
# KHÔNG phải 'x'@'localhost' (socket) — sai chỗ này là Access denied dù pass đúng.

if [ -f "$SRC/SHA256SUMS" ]; then
  echo "=== verify checksum"
  { command -v sha256sum >/dev/null && ( cd "$SRC" && sha256sum -c SHA256SUMS ) \
      || ( cd "$SRC" && shasum -a 256 -c SHA256SUMS ); } || { echo "❌ Checksum lệch — dump đã hỏng"; exit 1; }
fi

for base in "${TARGETS[@]}"; do
  f="$SRC/$base.sql"
  if [ $TEST -eq 1 ]; then db="restoretest_$base"; else db="$base"; fi

  exists=$($M -N -e "SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name='$db';")
  if [ $TEST -eq 1 ]; then
    [ "$exists" = "0" ] || { echo "❌ DB tạm $db đã tồn tại — từ chối (không tự DROP DB không phải của mình)"; exit 1; }
  else
    if [ "$exists" != "0" ]; then
      ntab=$($M -N -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='$db';")
      echo "⚠  $db đã tồn tại ($ntab bảng) — import sẽ GHI ĐÈ dữ liệu."
    fi
    if [ $YES -ne 1 ]; then
      read -rp "Gõ lại đúng tên DB '$db' để xác nhận restore: " ANS
      [ "$ANS" = "$db" ] || { echo "❌ Không khớp — huỷ, chưa thay đổi gì"; exit 1; }
    fi
  fi

  want=$(grep -c "^CREATE TABLE" "$f")
  echo "=== $db (mong đợi $want bảng)"

  $M -e "CREATE DATABASE IF NOT EXISTS \`$db\`
         CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;" || exit 1

  $M "$db" < "$f" || { echo "  ❌ IMPORT LỖI — DỪNG"; exit 1; }

  got=$($M -N -e "SELECT COUNT(*) FROM information_schema.tables
                  WHERE table_schema='$db' AND table_type='BASE TABLE';")
  if [ "$want" != "$got" ]; then
    echo "  ❌ lệch bảng: mong đợi $want, thực tế $got — DỪNG"; exit 1
  fi

  vw=$($M -N -e "SELECT COUNT(*) FROM information_schema.views WHERE table_schema='$db';")
  echo "  ✅ $got bảng, $vw view"

  [ $TEST -eq 1 ] && $M -e "DROP DATABASE \`$db\`;"
done

echo
if [ $TEST -eq 1 ]; then
  echo "✅ RESTORE-TEST PASS — dump dùng được, đã dọn sạch restoretest_*"
else
  echo "✅ IMPORT XONG"
fi
