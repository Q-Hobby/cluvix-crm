# Script deploy phía server

Mẫu `~/deploy/<instance>.sh` cho cluvix-crm. Quy trình + lý do từng bước: `../SKILL.md` §3.

- [1. Biến cần đổi khi tạo instance](#1-biến-cần-đổi-khi-tạo-instance)
- [2. Script](#2-script)

---

## 1. Biến cần đổi khi tạo instance

| Biến | Giá trị |
|---|---|
| `INSTANCE` | tên instance — trùng thư mục `<BASE>/<instance>` và `S3_FOLDER` |
| `SOURCE`, `INSTANCE_DIR` | đổi `<BASE>` thành thư mục web gốc thật trên server |
| `SERVICE` | tên unit systemd của instance |
| `LOG_DIR` | thư mục `StandardError=append:` trong unit systemd |

Script KHÔNG chứa bí mật — mọi cấu hình nằm trong `backend/.env` trên server.

## 2. Script

```bash
#!/bin/bash
# Deploy cluvix-crm (github.com/cluvix/cluvix-crm, nhánh main)
#
# Backend: pull + build ngay trên server (go-webp dùng cgo ⇒ không cross-compile từ macOS).
# Frontend: dist KHÔNG nằm trong repo ⇒ build trên máy dev rồi rsync vào $FE_UPLOAD trước khi chạy
#   (script local trên máy dev làm cả 2 bước). $FE_UPLOAD trống ⇒ giữ FE cũ.
export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin

INSTANCE="crm"
BRANCH="main"
SOURCE="<BASE>/cluvix-crm"
INSTANCE_DIR="<BASE>/${INSTANCE}"
BACKEND_DIR="${INSTANCE_DIR}/backend"
PUBLIC_DIR="${INSTANCE_DIR}/public"
FE_UPLOAD="${INSTANCE_DIR}/frontend-upload"
SERVICE="cluvix-crm"
BINARY="cluvixCrm"
LOG_DIR="<LOG_DIR>"

echo "=> [${INSTANCE}] Đang pull code..."
cd "${SOURCE}" || exit 1
if ! git pull --ff-only origin "${BRANCH}"; then
    echo "=> ERROR: Pull thất bại!"
    exit 1
fi
echo "=> Pull thành công! Commit: $(git log --oneline -1)"

echo "=> [${INSTANCE}] Đang build..."
cd "${SOURCE}/backend" || exit 1
if ! go build -trimpath -o "${BINARY}" .; then
    echo "=> ERROR: Build thất bại!"
    exit 1
fi
if ! go build -trimpath -o migrate ./cmd/migrate; then
    echo "=> ERROR: Build migrate thất bại!"
    exit 1
fi
echo "=> Build thành công!"

echo "=> [${INSTANCE}] Stop service trước khi copy..."
sudo systemctl stop "${SERVICE}" 2>/dev/null || true

echo "=> [${INSTANCE}] Backup binary cũ..."
[ -f "${BACKEND_DIR}/${BINARY}" ] && cp "${BACKEND_DIR}/${BINARY}" "${BACKEND_DIR}/${BINARY}.bak"
[ -f "${BACKEND_DIR}/migrate" ] && cp "${BACKEND_DIR}/migrate" "${BACKEND_DIR}/migrate.bak"

echo "=> [${INSTANCE}] Copy binary..."
cp "${SOURCE}/backend/${BINARY}" "${BACKEND_DIR}/${BINARY}"
cp "${SOURCE}/backend/migrate" "${BACKEND_DIR}/migrate"
chmod +x "${BACKEND_DIR}/${BINARY}" "${BACKEND_DIR}/migrate"

echo "=> [${INSTANCE}] Copy frontend..."
if [ -f "${FE_UPLOAD}/index.html" ]; then
    # Không --delete: tab đang mở bản cũ vẫn tải được chunk hash cũ.
    rsync -a "${FE_UPLOAD}/" "${PUBLIC_DIR}/"
    echo "=> Frontend đã cập nhật từ ${FE_UPLOAD}"
else
    echo "=> WARNING: ${FE_UPLOAD} chưa có bản build — giữ nguyên frontend hiện tại."
fi

echo "=> [${INSTANCE}] Symlink migrations..."
if [ ! -L "${BACKEND_DIR}/migrations" ]; then
    ln -s "${SOURCE}/backend/migrations" "${BACKEND_DIR}/migrations"
fi

rollback_binary() {
    echo "=> [${INSTANCE}] Đang rollback về binary trước đó..."
    if [ -f "${BACKEND_DIR}/${BINARY}.bak" ]; then
        cp "${BACKEND_DIR}/${BINARY}.bak" "${BACKEND_DIR}/${BINARY}"
        [ -f "${BACKEND_DIR}/migrate.bak" ] && cp "${BACKEND_DIR}/migrate.bak" "${BACKEND_DIR}/migrate"
        chmod +x "${BACKEND_DIR}/${BINARY}" "${BACKEND_DIR}/migrate"
        sudo systemctl restart "${SERVICE}" && echo "=> Rollback thành công, service đã chạy lại bản cũ." || echo "=> ERROR: Rollback cũng thất bại, cần can thiệp thủ công!"
    else
        echo "=> Không có backup để rollback!"
    fi
    sudo systemctl status "${SERVICE}" --no-pager
}

# App KHÔNG tự migrate (backend/DEPLOY.md mục 1) ⇒ migrate lỗi thì dừng, không bật bản mới trên schema cũ.
echo "=> [${INSTANCE}] Đang chạy migrate..."
cd "${BACKEND_DIR}" || exit 1
if ! ./migrate -cmd=up; then
    echo "=> ERROR: Migrate thất bại!"
    rollback_binary
    exit 1
fi
echo "=> Migrate hoàn tất!"

echo "=> [${INSTANCE}] Đang restart service ${SERVICE}..."
if ! sudo systemctl restart "${SERVICE}"; then
    echo "=> ERROR: Restart thất bại!"
    rollback_binary
    exit 1
fi
echo "=> Restart thành công!"

echo "=> [${INSTANCE}] Đang health-check..."
SERVER_PORT="$(grep -m1 '^SERVER_PORT=' "${BACKEND_DIR}/.env" | cut -d= -f2)"
SERVER_PORT="${SERVER_PORT:-8097}"
HEALTH_OK=0
for i in 1 2 3 4 5 6 7 8 9 10; do
    sleep 2
    if curl -sf -o /dev/null "http://127.0.0.1:${SERVER_PORT}/health/ready"; then
        HEALTH_OK=1
        break
    fi
    echo "=> Health-check lần ${i} chưa OK, thử lại..."
done

if [ "${HEALTH_OK}" -ne 1 ]; then
    echo "=> ERROR: Health-check thất bại sau khi restart! Log: ${LOG_DIR}/${INSTANCE}-error.log"
    tail -20 "${LOG_DIR}/${INSTANCE}-error.log" 2>/dev/null
    rollback_binary
    exit 1
fi
echo "=> Health-check OK!"

echo "=> Deploy [${INSTANCE}] hoàn tất!"
```
