#!/bin/sh
# run_qa.sh — cửa DUY NHẤT để agent (BMAD Dev/QA, subagent fan-out) chạy SERVER THẬT.
#
# VÌ SAO CÓ FILE NÀY (sự cố 2026-08-17):
#   Agent chạy `go run .` / `./tmp/main` → binary đọc SERVER_PORT=8011 từ .env → giành port
#   của air mà user đang chạy. Tệ hơn: scripts/run_dev.sh CỐ Ý không kill process lạ, nó
#   `exit 1` — nên air KHÔNG tự phục hồi, dev server chết hẳn tới khi user kill tay.
#   Lưu ý `go test` KHÔNG gây ra chuyện này (test dùng httptest.NewServer = port ephemeral);
#   thủ phạm luôn là việc chạy BINARY APP.
#
# HỢP ĐỒNG:
#   - Port QA cố định 8089 (8011 = của user, script này TỪ CHỐI chạy trên đó).
#   - Binary build ra ./.qa/, KHÔNG BAO GIỜ ./tmp/main (đó là binary air đang chạy —
#     ghi đè vào đó làm air restart vào binary của agent).
#   - Chạy xong (thoát bình thường, Ctrl-C, bị kill, hay hết --timeout) → tự kill process
#     con + xoá binary. Không để rác, không để process mồ côi giữ port.
#
# DÙNG:
#   scripts/run_qa.sh                      # chạy foreground, tự tắt sau 120s
#   scripts/run_qa.sh --timeout 300        # đổi hạn tự tắt
#   scripts/run_qa.sh --port 8088          # đổi port (KHÔNG được là 8011)
#   Agent nên chạy nền rồi curl vào http://localhost:8089/... , xong thì kill script.
#   Hết hạn --timeout là LƯỚI AN TOÀN, không phải cách dừng bình thường.

set -e

DEV_PORT=8011        # port của user (air) — bất khả xâm phạm
QA_PORT=8089
TIMEOUT=120

while [ $# -gt 0 ]; do
  case "$1" in
    --port)    QA_PORT="${2:-}"; shift 2 ;;
    --timeout) TIMEOUT="${2:-}"; shift 2 ;;
    -h|--help) sed -n '2,26p' "$0"; exit 0 ;;
    *) echo "run_qa.sh: tham số lạ '$1' (xem --help)" >&2; exit 2 ;;
  esac
done

# backend/ dù gọi từ đâu
BACKEND_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$BACKEND_DIR"

# PHẢI nằm trong ./tmp: air có exclude_dir=["tmp",...] nên build/ghi ở đây KHÔNG kích rebuild.
# Đã thử ./.qa và HỎNG (2026-08-17): air watch cả THƯ MỤC MỚI, không chỉ file .go — `mkdir ./.qa`
# làm air rebuild → kill server → run_dev.sh thấy port còn bị con đang chết giữ → exit 1 → dev
# server mất hẳn. Đúng cái sự cố script này sinh ra để chặn.
# (clean_on_exit của air xoá tmp/ khi air thoát — vô hại: process đang chạy giữ inode của binary,
#  và vòng dọn rác bên dưới đã chịu được file biến mất.)
QA_DIR=./tmp/qa
BIN=$QA_DIR/qa-$$
LOG=$QA_DIR/qa-$$.log
CHILD=""

# Validate số: `--timeout abc` mà không chặn sẽ nổ ở phép so sánh số học tận cuối script,
# sau khi đã build + khởi động server (thông báo lỗi lúc đó vô nghĩa với người đọc).
case "$QA_PORT" in ''|*[!0-9]*) echo "run_qa.sh: --port phải là số, nhận '$QA_PORT'" >&2; exit 2 ;; esac
case "$TIMEOUT" in ''|*[!0-9]*) echo "run_qa.sh: --timeout phải là số giây, nhận '$TIMEOUT'" >&2; exit 2 ;; esac
if [ "$QA_PORT" -lt 1024 ] || [ "$QA_PORT" -gt 65535 ]; then
  echo "run_qa.sh: --port ngoài khoảng hợp lệ 1024-65535: $QA_PORT" >&2; exit 2
fi

if [ "$QA_PORT" = "$DEV_PORT" ]; then
  echo "run_qa.sh: TỪ CHỐI chạy trên port $DEV_PORT — đó là port air của user. Port QA mặc định là 8089." >&2
  exit 1
fi

cleanup() {
  if [ -n "$CHILD" ] && kill -0 "$CHILD" 2>/dev/null; then
    kill "$CHILD" 2>/dev/null || true
    # cho nó 3s tự đóng listener rồi mới cưỡng chế
    i=0
    while [ "$i" -lt 30 ] && kill -0 "$CHILD" 2>/dev/null; do
      i=$((i + 1)); sleep 0.1
    done
    kill -9 "$CHILD" 2>/dev/null || true
  fi
  rm -f "$BIN"
  # Chỉ báo "đã dừng server" khi thật sự có server chạy — build fail thì chưa có gì để dừng.
  if [ -n "$CHILD" ]; then
    echo "run_qa.sh: đã dừng server QA + xoá binary $BIN (log giữ lại: $LOG)"
  else
    echo "run_qa.sh: dọn dẹp (chưa kịp khởi động server)"
  fi
}
trap 'cleanup' EXIT
trap 'exit 143' INT TERM   # để trap EXIT chạy cleanup

mkdir -p "$QA_DIR"

# Dọn rác của lần chạy trước (agent bị SIGKILL thì trap EXIT không chạy).
# Chỉ xoá artifact có PID trong tên đã chết — không đụng của instance đang sống.
for f in "$QA_DIR"/qa-*; do
  [ -e "$f" ] || continue
  pid=$(basename "$f" | sed -e 's/^qa-//' -e 's/\.log$//')
  case "$pid" in
    ''|*[!0-9]*) continue ;;
  esac
  kill -0 "$pid" 2>/dev/null || rm -f "$f"
done

# KHÔNG tự kill process lạ đang giữ port (cùng triết lý run_dev.sh: giành port âm thầm
# sẽ đẩy chủ sở hữu vào vòng crash-restart).
# -sTCP:LISTEN: chỉ tính process ĐANG LISTEN. Không có flag này thì một socket client
# còn sót (curl/trình duyệt nối tới port QA) cũng bị coi là "port bận" → từ chối chạy oan.
HOLDER=$(lsof -ti :"$QA_PORT" -sTCP:LISTEN 2>/dev/null | head -1 || true)
if [ -n "$HOLDER" ]; then
  echo "run_qa.sh: port $QA_PORT đang bị PID $HOLDER giữ — dừng, KHÔNG tự kill." >&2
  echo "  Kiểm tra: lsof -i :$QA_PORT   (nhiều khả năng là run_qa.sh của agent khác chưa dọn)" >&2
  exit 1
fi

# Cảnh báo hiện DB đang trỏ tới — không chặn (user chốt 2026-08-17: DB_NAME local đã là DB test),
# nhưng phải NHÌN THẤY chứ không giấu: server QA ghi thật vào DB này.
# DB_NAME/DB_HOST do caller export PHẢI THẮNG .env (sửa 2026-08-19). Trước đây script gán đè vô
# điều kiện: agent chạy `DB_NAME=xxx_tmp run_qa.sh` để tránh DB dev vẫn bị kéo ngược về DB thật —
# im lặng, đúng lớp sự cố CATASTROPHIC trong CLAUDE.md. godotenv.Load() không override env đã set,
# nên export ở đây là đủ để binary đi vào DB dùng-một-lần.
DB_NAME="${DB_NAME:-$(grep -E '^DB_NAME=' .env 2>/dev/null | head -1 | cut -d= -f2- || true)}"
DB_HOST="${DB_HOST:-$(grep -E '^DB_HOST=' .env 2>/dev/null | head -1 | cut -d= -f2- || true)}"
export DB_NAME DB_HOST
echo "run_qa.sh: DB đang dùng = ${DB_NAME:-?} @ ${DB_HOST:-?}  (server QA GHI THẬT vào đây)"

echo "run_qa.sh: build → $BIN"
go build -o "$BIN" .

# godotenv.Load() KHÔNG override biến môi trường đã set → SERVER_PORT ở đây thắng .env.
SERVER_PORT="$QA_PORT" "$BIN" >> "$LOG" 2>&1 &
CHILD=$!

# Chờ server thật sự lắng nghe (build xong ≠ bind xong; agent curl sớm sẽ ăn connection refused).
READY=0
i=0
while [ "$i" -lt 100 ]; do
  i=$((i + 1))
  if ! kill -0 "$CHILD" 2>/dev/null; then
    echo "run_qa.sh: server chết khi khởi động — 30 dòng cuối log:" >&2
    tail -30 "$LOG" >&2 || true
    exit 1
  fi
  if lsof -ti :"$QA_PORT" -sTCP:LISTEN >/dev/null 2>&1; then
    READY=1
    break
  fi
  sleep 0.2
done

# Cạn vòng chờ mà chưa bind → PHẢI báo lỗi. Trước đây code rơi thẳng xuống dòng "sẵn sàng"
# (sửa 2026-08-17): script nói dối thì agent curl vào ăn connection refused rồi đi debug nhầm chỗ.
if [ "$READY" -ne 1 ]; then
  echo "run_qa.sh: HẾT 20s mà server chưa bind port $QA_PORT — KHÔNG sẵn sàng. 30 dòng cuối log:" >&2
  tail -30 "$LOG" >&2 || true
  exit 1
fi

echo "run_qa.sh: server QA sẵn sàng tại http://localhost:$QA_PORT (PID $CHILD, tự tắt sau ${TIMEOUT}s) — log: $LOG"

# Foreground tới khi: hết TIMEOUT, server chết, hoặc script bị kill.
elapsed=0
while [ "$elapsed" -lt "$TIMEOUT" ]; do
  kill -0 "$CHILD" 2>/dev/null || { echo "run_qa.sh: server tự thoát, xem $LOG" >&2; exit 1; }
  sleep 1
  elapsed=$((elapsed + 1))
done

echo "run_qa.sh: hết hạn ${TIMEOUT}s — tắt server QA."
