#!/bin/zsh
# verify_integration_isolated.sh — chạy integration test ĐÚNG CÁCH: mỗi package MỘT DB dùng-một-lần
# RIÊNG, TUẦN TỰ, và coi "có test bị skip" là FAIL.
#
# ⛔ VÌ SAO CẦN (sự cố đo được 2026-08-18, story-13 zalo-personal-core):
#   `go test -tags=integration ./<module>/...` chạy nhiều package SONG SONG trên CÙNG một
#   TEST_MYSQL_DSN. Mỗi TestMain gọi testhelpers.New = dropAllTables + migrate, nên chúng ĐÈ NHAU →
#   testhelpers.New lỗi → testDB = nil → MỌI test trong package đó t.Skip → in ra "ok".
#   Output KHÔNG có bất kỳ dấu hiệu nào. Đã xảy ra thật:
#       ./omnichat/...       → ok handler 4.0s
#       ./omnichat/handler/  → FAIL (panic)
#   ⇒ "integration PASS" từ lệnh chạy nhiều package chung DSN là KẾT QUẢ RỖNG, không được tin.
#
# ⛔ CẤM trỏ vào DB thật: script tự tạo DB tên `itv_<n>` rồi DROP. KHÔNG dùng DB_NAME trong .env.
#
# Dùng:  backend/scripts/verify_integration_isolated.sh <pkg> [<pkg> ...]
# Ví dụ: backend/scripts/verify_integration_isolated.sh ./internal/modules/webapp/omnichat/service \
#                                                       ./internal/modules/webapp/omnichat/handler
# Env:   MYSQL_HOST/MYSQL_PORT/MYSQL_USER/MYSQL_PASS (mặc định localhost/3306/root + hỏi nếu trống).
set -u

: ${MYSQL_HOST:=127.0.0.1}
: ${MYSQL_PORT:=3306}
: ${MYSQL_USER:=root}
: ${MYSQL_PASS:=}
: ${MYSQL_BIN:=mysql}
# ITV_PREFIX — tiền tố tên DB tạm. Đổi khi chạy NHIỀU phiên song song (vd nhiều QA agent) để không
# giành nhau tên `itv_1`, `itv_2`... Mỗi phiên một prefix riêng là đủ cách ly.
: ${ITV_PREFIX:=itv}

if [ $# -eq 0 ]; then
  echo "usage: $0 <package> [<package> ...]" >&2
  exit 2
fi
if [ -z "$MYSQL_PASS" ]; then
  echo "!! đặt MYSQL_PASS (mật khẩu MySQL local) trước khi chạy" >&2
  exit 2
fi

myq() { "$MYSQL_BIN" -h"$MYSQL_HOST" -P"$MYSQL_PORT" -u"$MYSQL_USER" -p"$MYSQL_PASS" -e "$1" 2>/dev/null; }

FAILED=0
i=0
for p in "$@"; do
  i=$((i+1))
  DB="${ITV_PREFIX}_${i}_test"
  myq "DROP DATABASE IF EXISTS $DB; CREATE DATABASE $DB CHARACTER SET utf8mb4;"
  DSN="$MYSQL_USER:$MYSQL_PASS@($MYSQL_HOST:$MYSQL_PORT)/$DB?charset=utf8mb4&parseTime=True&loc=Local&multiStatements=true"
  # DB_NAME= : vô hiệu guard so-trùng-DB-thật của testhelpers vì DSN ở đây chắc chắn là DB tạm.
  OUT=$(TEST_MYSQL_DSN="$DSN" DB_NAME= GOFLAGS=-p=4 go test -count=1 -tags=integration -v "$p" 2>&1)
  RUN=$(print -r -- "$OUT" | grep -c "^=== RUN")
  SKIP=$(print -r -- "$OUT" | grep -c "integration DB not available")
  if print -r -- "$OUT" | grep -qE "^(FAIL|--- FAIL|panic)"; then
    echo "FAIL  $p  (run=$RUN skip=$SKIP)"
    print -r -- "$OUT" | grep -E "^--- FAIL|panic:" | head -5
    FAILED=1
  elif [ "$RUN" -eq 0 ]; then
    echo "FAIL  $p  — KHÔNG test nào chạy (build tag sai? package rỗng?)"
    FAILED=1
  elif [ "$SKIP" -gt 0 ]; then
    echo "FAIL  $p  (run=$RUN skip=$SKIP) — CÓ TEST SKIP ⇒ kết quả xanh KHÔNG đầy đủ"
    FAILED=1
  else
    echo "ok    $p  (run=$RUN skip=$SKIP)"
  fi
  myq "DROP DATABASE IF EXISTS $DB;"
done
exit $FAILED
