#!/usr/bin/env bash
# So SCHEMA của một DB đang chạy với DB dựng TỪ MIGRATION, 4 chiều: bảng · cột · chỉ mục · FK.
#
# Vì sao cần: bench hiệu năng thường clone bằng `mysqldump` từ DB dev — nên nó mang theo cả DRIFT
# của dev. Ngày 2026-09-09 việc này đã dẫn tới 2 sai lầm thật trong MỘT phiên:
#   1. Kết luận "thiếu chỉ mục `(technical_id, confirm_time)`" cho treatment_course_technical —
#      thực ra DB dựng-từ-migration ĐÃ có, chỉ dev bị ĐẢO THỨ TỰ CỘT.
#   2. Đăng ký `warehouse` vào `branchTables` vì dev có cột `branch_id` — DB dựng-từ-migration chỉ
#      có `branchId` (camelCase) ⇒ suýt ship lỗi 1054 cho MỌI DB khách.
#
# Dùng:  bash verify-schema-drift.sh <db_dang_chay> [db_tam]
#   vd:  bash skill/skills/_tooling/verify-schema-drift.sh cluvix_crm
#
# KHÔNG ghi gì vào DB được so. DB tham chiếu là DB dùng-một-lần (tên kết thúc `_tmp`), tự drop.
set -euo pipefail

SRC="${1:?thiếu tên DB cần kiểm}"
REF="${2:-schema_drift_ref_tmp}"
case "$REF" in *_tmp|*_test) ;; *) echo "DB tham chiếu phải kết thúc _tmp hoặc _test"; exit 1;; esac

BE="$(cd "$(dirname "$0")/../../.." && pwd)/backend"
[ -d "$BE" ] || BE="$(pwd)/backend"
export MYSQL_PWD="$(grep -E '^DB_PASS=' "$BE/.env" | cut -d= -f2-)"
DBNAME="$(grep -E '^DB_NAME=' "$BE/.env" | cut -d= -f2-)"
[ "$REF" = "$DBNAME" ] && { echo "DB tham chiếu trùng DB_NAME thật — DỪNG"; exit 1; }

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"; mysql -uroot -e "DROP DATABASE IF EXISTS \`$REF\`" >/dev/null 2>&1 || true' EXIT

echo "→ dựng DB tham chiếu $REF từ migration…"
mysql -uroot -e "DROP DATABASE IF EXISTS \`$REF\`; CREATE DATABASE \`$REF\` DEFAULT CHARACTER SET utf8mb4;"
(cd "$BE" && go run cmd/migrate/main.go -cmd=up -db="$REF" >/dev/null)

snap() { # $1=db  $2=nhãn
  mysql -uroot -N -e "SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA='$1' AND TABLE_TYPE='BASE TABLE' ORDER BY 1" > "$WORK/tables_$2"
  mysql -uroot -N -e "SELECT CONCAT_WS('|',TABLE_NAME,COLUMN_NAME,COLUMN_TYPE,IS_NULLABLE,IFNULL(COLUMN_DEFAULT,'~'),EXTRA) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='$1' ORDER BY 1" > "$WORK/cols_$2"
  mysql -uroot -N --init-command="SET SESSION group_concat_max_len=100000" -e "SELECT CONCAT_WS('|',TABLE_NAME,INDEX_NAME,GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX),NON_UNIQUE) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA='$1' GROUP BY TABLE_NAME,INDEX_NAME,NON_UNIQUE ORDER BY 1" > "$WORK/idx_$2"
  mysql -uroot -N -e "SELECT CONCAT_WS('|',k.TABLE_NAME,k.COLUMN_NAME,k.REFERENCED_TABLE_NAME,r.DELETE_RULE) FROM information_schema.KEY_COLUMN_USAGE k JOIN information_schema.REFERENTIAL_CONSTRAINTS r ON r.CONSTRAINT_NAME=k.CONSTRAINT_NAME AND r.CONSTRAINT_SCHEMA=k.CONSTRAINT_SCHEMA WHERE k.TABLE_SCHEMA='$1' AND k.REFERENCED_TABLE_NAME IS NOT NULL ORDER BY 1" > "$WORK/fk_$2"
}
snap "$SRC" src; snap "$REF" ref

# ĐỐI CHỨNG: không có mốc này thì snapshot rỗng cũng ra "0 khác biệt"
n=$(wc -l < "$WORK/tables_src"); [ "$n" -gt 50 ] || { echo "chỉ đọc được $n bảng từ $SRC — phép so vô nghĩa"; exit 1; }
echo "đối chứng: $SRC có $n bảng · $REF có $(wc -l < "$WORK/tables_ref") bảng"

comm -12 "$WORK/tables_src" "$WORK/tables_ref" > "$WORK/common"
only() { awk -F'|' 'NR==FNR{t[$1];next} $1 in t' "$WORK/common" "$1"; }
rc=0
say() { echo; echo "=== $1 ==="; }

say "BẢNG chỉ có ở $SRC";  comm -23 "$WORK/tables_src" "$WORK/tables_ref" | sed 's/^/  /'
say "BẢNG chỉ có ở migration (${SRC} THIẾU)"; comm -13 "$WORK/tables_src" "$WORK/tables_ref" | sed 's/^/  /'

for k in cols idx fk; do
  only "$WORK/${k}_src" | sort > "$WORK/${k}_s"; only "$WORK/${k}_ref" | sort > "$WORK/${k}_r"
  a=$(comm -23 "$WORK/${k}_s" "$WORK/${k}_r" | wc -l | tr -d ' ')
  b=$(comm -13 "$WORK/${k}_s" "$WORK/${k}_r" | wc -l | tr -d ' ')
  say "$k: $a dòng chỉ ở $SRC · $b dòng chỉ ở migration"
  comm -23 "$WORK/${k}_s" "$WORK/${k}_r" | sed 's/^/  chỉ-'"$SRC"': /'
  comm -13 "$WORK/${k}_s" "$WORK/${k}_r" | sed 's/^/  chỉ-migration: /'
  [ "$a" != "0" ] || [ "$b" != "0" ] && rc=1
done

# Lớp nguy hiểm nhất: CÙNG TÊN chỉ mục nhưng KHÁC danh sách cột — code/kết luận hiệu năng dựa vào
# tên sẽ đúng ở DB này và sai ở DB kia mà không ai thấy.
say "CHỈ MỤC cùng tên, KHÁC cột (lớp bug tct 2026-09-09)"
join -t'|' -j1 <(awk -F'|' '{print $1"."$2"|"$3}' "$WORK/idx_s" | sort) \
               <(awk -F'|' '{print $1"."$2"|"$3}' "$WORK/idx_r" | sort) \
  | awk -F'|' '$2!=$3 {printf "  %s\n      %s = %s\n      migration = %s\n", $1, "'"$SRC"'", $2, $3}'

echo; [ "$rc" = "0" ] && echo "✅ KHÔNG có drift" || echo "⚠ CÓ drift — đọc từng mục ở trên"
exit 0
