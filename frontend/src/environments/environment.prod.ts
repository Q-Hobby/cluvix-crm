export const environment = {
  prod: true,
  option: 3,
  apiUrl: '/api',
  // Key PrimeUI (Community miễn phí nếu đủ điều kiện, hoặc Commercial) — cách đăng ký: README §2.
  // Repo public để RỖNG, KHÔNG commit key thật (điều khoản PrimeUI cấm công bố key cho người khác dùng).
  primeuiLicense: '', // build prod có key: dùng environment.cluvix.ts (gitignored) + `npm run build:cluvix` (README §2.3), đừng sửa file này
  // URL file JSON chứa key PrimeUI trên CDN. Repo public để RỖNG ⇒ không fetch CDN. Bản Cluvix đặt trong environment.cluvix.ts (gitignored).
  primeuiLicenseCdnUrl: '',
};
