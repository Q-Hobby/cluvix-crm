# Dev image — chạy `ng serve` với source mount từ host, KHÔNG phải build production.
# Node: Angular 22 cần ^20.19.0 || ^22.12.0 || >=24.0.0 → dùng node 22 LTS.
FROM node:22-alpine

WORKDIR /app

COPY package.json package-lock.json .npmrc ./
# HUSKY=0: script `prepare` (husky install) cần .git, container không có.
RUN HUSKY=0 npm ci

COPY . .

EXPOSE 4211

# Cấu hình "docker": environment.docker.ts (apiUrl '/api') + proxy /api → http://backend:8011.
# Không SSL trong container. --poll vì file watch qua bind mount (macOS/Windows) không nhận event.
CMD ["npx", "ng", "serve", "--configuration", "docker", "--host", "0.0.0.0", "--port", "4211", "--poll", "1000"]
