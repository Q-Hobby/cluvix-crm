# Dev image — chạy `go run` với source mount từ host, KHÔNG phải build production.
# Build production dùng `make` (xem backend/Makefile / backend/DEPLOY.md), không qua Dockerfile này.
# Phiên bản Go bám backend/go.mod (go 1.26.x).
FROM golang:1.26-alpine

# mysql-client: cung cấp `mysqladmin` cho docker-entrypoint.sh đợi MySQL sẵn sàng.
RUN apk add --no-cache mysql-client git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

EXPOSE 8011

# Gọi qua "sh" thay vì exec trực tiếp — không phụ thuộc exec bit của file, vì
# docker-compose mount ./backend:/app đè toàn bộ /app (kể cả permission) lúc
# runtime, làm mất tác dụng mọi `chmod +x` chạy lúc build image.
ENTRYPOINT ["sh", "docker-entrypoint.sh"]
