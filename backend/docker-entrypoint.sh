#!/bin/sh
set -e

# Container KHÔNG dùng file backend/.env: toàn bộ cấu hình đến từ biến môi trường
# (docker-compose.yml + .env.docker). main.go và cmd/migrate gọi godotenv.Load()
# nhưng bỏ qua lỗi khi thiếu file, và biến môi trường luôn thắng giá trị trong file.

DB_PORT="${DB_PORT:-3306}"

echo "Đợi MySQL ($DB_HOST:$DB_PORT) sẵn sàng..."
# MYSQL_PWD thay vì -p"$DB_PASS" — password rỗng với -p sẽ kích hoạt hỏi
# tương tác, container không có TTY nên treo vô hạn thay vì báo lỗi.
export MYSQL_PWD="$DB_PASS"
until mysqladmin ping -h "$DB_HOST" -P "$DB_PORT" -u"$DB_USER" --silent 2>/dev/null; do
  sleep 1
done
echo "MySQL đã sẵn sàng"

echo "Chạy migration..."
go run ./cmd/migrate -cmd=up
echo "Migration xong"

exec go run .
