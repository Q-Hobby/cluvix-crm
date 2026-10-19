#!/bin/sh
# Wrapper dùng với air's full_bin: tự restart binary khi die, dừng khi air kill.
#
# SINGLE-INSTANCE (thêm 2026-08-10 sau sự cố crash-loop 733 lần status=137):
# PIDFILE dùng chung + 2 instance chạy song song = vòng lặp tự sát. Cơ chế hỏng:
#   A start con → ghi pid_A vào PIDFILE → B vào vòng lặp, đọc PIDFILE thấy pid_A đang
#   giữ 8011, tưởng là orphan của CHÍNH MÌNH → kill -9 pid_A → B start con, ghi pid_B →
#   A tỉnh dậy (con vừa chết), đọc PIDFILE thấy pid_B → kill -9 pid_B → ... lặp mãi ~1s.
# Triệu chứng: run.log đầy "binary exited (status=137)", server chớp tắt liên tục, RAM
# vẫn bình thường (KHÔNG phải OOM — đừng đi tìm nguyên nhân bộ nhớ).
# Nguyên nhân gốc là 2 instance, nên fix là khoá 1-instance chứ không phải đổi PIDFILE.

STOP=0
# State ĐẶT NGOÀI ./tmp (sự cố 2026-08-13): air có clean_on_exit=true nên khi air thoát
# nó xoá SẠCH tmp/ — kéo theo pidfile + lockdir. Instance mới mất hết chứng cứ sở hữu:
# lock trống → giành được, PIDFILE trống → LAST_PID="" → coi con của instance CŨ còn sót
# là "process lạ" → exit 1, air không bao giờ start được cho tới khi user tự kill tay.
# ./.run/ không nằm trong tmp_dir của air nên sống sót qua clean_on_exit.
RUNDIR=./.run
PIDFILE=$RUNDIR/run_dev.pid
LOCKDIR=$RUNDIR/run_dev.lock        # mkdir là atomic trên POSIX → dùng làm mutex
LOCK_HELD=0

mkdir -p ./tmp "$RUNDIR"

log() {
  echo "$1" >> ./tmp/run.log
  echo "$1"
}

release_lock() {
  # Chỉ xoá lock nếu CHÍNH MÌNH đang giữ — tránh xoá nhầm lock của instance đã tiếp quản.
  [ "$LOCK_HELD" -eq 1 ] || return 0
  if [ "$(cat "$LOCKDIR/owner" 2>/dev/null)" = "$$" ]; then
    rm -rf "$LOCKDIR"
  fi
  LOCK_HELD=0
}

# Giành quyền chạy theo luật INSTANCE MỚI NHẤT THẮNG (đổi 2026-10-19, trước đây instance mới
# tự thoát khi lock bận). Lý do: air v1.64.5 có race làm nó MẤT DẤU instance cũ — `start()` rút
# token của build đang chạy khỏi semaphore `buildRunCh`, build cũ khi return lại rút nhầm token
# của build mới ⇒ 2 `runBin` chạy chồng, cùng ghi đè `binStopCh` ⇒ một run_dev.sh không còn được
# air theo dõi, rebuild sau air không kill nó nữa (air chỉ kill process group nó đang giữ). Luật
# cũ "lock bận ⇒ thoát" biến race đó thành kẹt vĩnh viễn: instance mồ côi giữ lock + binary cũ,
# mọi instance mới air spawn đều thoát sau 10s (sự cố 2026-10-18 23:00 và 2026-10-19 09:10 — trang
# chủ 404 vì BE chạy binary trước khi có route mới). Instance mới luôn là cái air vừa build xong
# nên nó đúng là thứ cần chạy; vẫn chỉ MỘT vòng lặp sống (chủ cũ bị dừng hẳn, không restart lại)
# nên không tái sinh crash-loop 2026-08-10.
TAKEOVER_PID=""
TAKEOVER_TRIES=0
acquire_lock() {
  i=0
  while [ "$i" -lt 50 ]; do
    i=$((i + 1))
    # air gửi SIGINT trong lúc mình còn đang chờ lock ⇒ dừng ngay, đừng giành lock rồi mới thoát.
    [ "$STOP" -eq 1 ] && return 2
    if mkdir "$LOCKDIR" 2>/dev/null; then
      echo $$ > "$LOCKDIR/owner"
      LOCK_HELD=1
      return 0
    fi
    OWNER=$(cat "$LOCKDIR/owner" 2>/dev/null)
    # Lock mồ côi: chủ cũ bị SIGKILL nên trap EXIT không chạy → thu hồi.
    if [ -z "$OWNER" ] || ! kill -0 "$OWNER" 2>/dev/null; then
      log "[$(date '+%H:%M:%S')] run_dev.sh: thu hồi lock mồ côi của PID ${OWNER:-?}"
      rm -rf "$LOCKDIR"
      continue
    fi
    # PID trong owner đã bị hệ điều hành cấp lại cho tiến trình khác ⇒ cũng là lock mồ côi.
    # KHÔNG kill tiến trình đó — nó không phải của mình.
    case "$(ps -o command= -p "$OWNER" 2>/dev/null)" in
      *run_dev.sh*) ;;
      *)
        log "[$(date '+%H:%M:%S')] run_dev.sh: PID $OWNER trong lock không còn là run_dev.sh — thu hồi lock"
        rm -rf "$LOCKDIR"
        continue
        ;;
    esac
    # Tiếp quản: SIGTERM trước (trap của chủ cũ kill binary con rồi thoát, tự xoá lock); quá ~5s
    # (đang kẹt migrate/chờ lock) thì SIGKILL cả chủ cũ lẫn con trực tiếp của nó. Lấy danh sách
    # con TRƯỚC khi kill — chủ chết rồi thì con bị đổi cha sang PID 1, không tìm lại được.
    # Không kill theo process group: chạy tay từ shell thì group có thể là của chính shell đó.
    if [ "$OWNER" != "$TAKEOVER_PID" ]; then
      TAKEOVER_PID=$OWNER
      TAKEOVER_TRIES=0
      log "[$(date '+%H:%M:%S')] run_dev.sh: instance cũ PID $OWNER còn giữ lock (air đã mất dấu nó) — dừng nó để tiếp quản"
      kill -TERM "$OWNER" 2>/dev/null
    else
      TAKEOVER_TRIES=$((TAKEOVER_TRIES + 1))
      if [ "$TAKEOVER_TRIES" -eq 25 ]; then
        KIDS=$(pgrep -P "$OWNER" 2>/dev/null)
        log "[$(date '+%H:%M:%S')] run_dev.sh: PID $OWNER không dừng sau 5s — SIGKILL nó và con $(echo $KIDS)"
        kill -KILL "$OWNER" $KIDS 2>/dev/null
      fi
    fi
    sleep 0.2
  done
  return 1
}

trap 'STOP=1; kill "$CHILD" 2>/dev/null' INT TERM
trap 'release_lock' EXIT

acquire_lock
RC=$?
if [ "$RC" -eq 2 ]; then
  exit 0
elif [ "$RC" -ne 0 ]; then
  log "[$(date '+%H:%M:%S')] run_dev.sh: không tiếp quản được lock của PID $(cat "$LOCKDIR/owner" 2>/dev/null) sau 10s — thoát, KHÔNG giành port. Kiểm tra 'ps -p <PID>', kill tay rồi 'rm -rf ./.run/run_dev.lock'."
  exit 1
fi

# MIGRATE trước khi serve (thêm 2026-09-04 sau sự cố spam 600 alert Telegram):
# code mới + schema cũ = mọi INSERT/SELECT đụng cột mới fail 1054, và vì cron chạy nền
# nên lỗi lặp vô hạn chứ không dừng ở 1 lần. Lần đó `is_identity_verified` đã có trong
# entity Go nhưng migration chưa apply lên DB dev → 100 lỗi/5 phút suốt 25 phút.
#
# Đặt SAU acquire_lock và NGOÀI vòng while, có chủ đích:
#   - sau lock  → 2 instance chồng lấn lúc air rebuild không cùng migrate một DB;
#   - ngoài while → chạy 1 lần mỗi lượt start, không lặp lại ở mỗi lần binary crash-restart.
# Chi phí đo thật: ~0.1-0.3s khi cache Go ấm, no-op nếu không có migration pending.
#
# KHÔNG exit khi migrate fail — cố ý. Cả lịch sử file này là chống việc "air chết hẳn tới
# khi user kill tay"; chặn start vì migrate lỗi sẽ dựng lại đúng cái bẫy đó. Thay vào đó
# hét thật to rồi vẫn serve, để người sửa được migration mà không mất dev server.
TS=$(date '+%H:%M:%S')
if MIGRATE_OUT=$(go run cmd/migrate/main.go -cmd=up 2>&1); then
  log "[$TS] run_dev.sh: migrate OK — $(echo "$MIGRATE_OUT" | tail -1)"
else
  log "[$TS] ⚠️⚠️ run_dev.sh: MIGRATE THẤT BẠI — server vẫn chạy nhưng SCHEMA CÓ THỂ CŨ HƠN CODE."
  log "$MIGRATE_OUT"
  log "[$TS] ⚠️⚠️ Sửa migration rồi lưu lại file .go bất kỳ để air restart (hoặc chạy tay: go run cmd/migrate/main.go -cmd=up)."
fi

while [ $STOP -eq 0 ]; do
  # air's clean_on_exit có thể đã xoá thư mục tmp/ ở lần chạy trước
  mkdir -p ./tmp "$RUNDIR"

  # Chỉ tự kill process trên port 8011 nếu đó là chính con của run_dev.sh
  # (orphan còn sót từ lần chạy trước) — không đụng vào process lạ (vd agent/test
  # tự bind port này), để tránh giành port âm thầm rồi vào vòng lặp crash-restart.
  LAST_PID=""
  [ -f "$PIDFILE" ] && LAST_PID=$(cat "$PIDFILE" 2>/dev/null)

  # -sTCP:LISTEN BẮT BUỘC (sự cố 2026-08-28): `lsof -ti :8011` khớp MỌI socket có port
  # 8011 ở bất kỳ đầu nào — kể cả đầu CLIENT. Chrome đang mở app (SSE + keep-alive) giữ
  # 127.0.0.1:5xxxx->127.0.0.1:8011 sau khi server cũ chết → lsof trả PID Chrome → không
  # khớp PIDFILE → nhánh "process lạ" → exit 1, air chết hẳn dù port thật sự TRỐNG.
  for pid in $(lsof -ti :8011 -sTCP:LISTEN 2>/dev/null); do
    if [ -n "$LAST_PID" ] && [ "$pid" = "$LAST_PID" ]; then
      kill -9 "$pid" 2>/dev/null
    else
      TS=$(date '+%H:%M:%S')
      # Vẫn KHÔNG tự kill (process lạ có thể là test/agent khác), nhưng chỉ thẳng thủ phạm:
      # nếu cha của nó là một run_dev.sh mồ côi thì user cần kill CHA trước, kill con vô ích
      # (cha restart lại sau 1s).
      PPID_OF=$(ps -o ppid= -p "$pid" 2>/dev/null | tr -d ' ')
      PCMD=$(ps -o command= -p "${PPID_OF:-0}" 2>/dev/null)
      case "$PCMD" in
        *run_dev.sh*)
          log "[$TS] Port 8011 đang bị giữ bởi PID $pid — con của một run_dev.sh CŨ (PID $PPID_OF). Kill tiến trình CHA trước: 'kill $PPID_OF' (kill mỗi $pid vô ích, cha sẽ restart sau 1s)."
          ;;
        *)
          log "[$TS] Port 8011 đang bị giữ bởi PID $pid (không phải tiến trình cũ của run_dev.sh) — dừng lại, không tự kill. Kiểm tra 'lsof -i :8011' rồi xử lý thủ công."
          ;;
      esac
      exit 1
    fi
  done
  sleep 0.2

  # Kiểm lại ngay trước khi spawn (đo 2026-10-19): /bin/sh của macOS là bash 3.2, signal tới
  # đúng lúc nó fork command substitution `$(lsof …)`/`$(cat …)` phía trên thì trap chạy trong
  # SUBSHELL — STOP=1 nằm ở tiến trình con, tiến trình chính mất cờ và vẫn start binary. Check
  # này thu hẹp cửa sổ; lưới cuối là SIGKILL sau 5s (air: kill_delay; tiếp quản: acquire_lock).
  [ $STOP -eq 1 ] && break

  ./tmp/main >> ./tmp/run.log 2>&1 &
  CHILD=$!
  echo "$CHILD" > "$PIDFILE"
  wait "$CHILD"
  STATUS=$?

  [ $STOP -eq 1 ] && break

  TS=$(date '+%H:%M:%S')
  log "[$TS] binary exited (status=$STATUS), restarting in 1s..."
  sleep 1
done
