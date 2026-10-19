// Dùng bởi cấu hình `docker` (docker-compose). Trình duyệt gọi cùng origin `/api`,
// dev-server Angular proxy sang backend (xem frontend/proxy.docker.conf.json).
export const environment = {
  prod: false,
  option: 3,
  apiUrl: '/api',
  // Repo public để RỖNG — không có key ⇒ PrimeNG chỉ hiện banner đỏ, không hỏng chức năng.
  // Có key thì dán vào bản copy cục bộ (xem DOCKER.md).
  primeuiLicense: '',
  primeuiLicenseCdnUrl: '',
};
