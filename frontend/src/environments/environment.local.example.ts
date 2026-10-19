export const environment = {
  prod: false,
  option: 3,
  apiUrl: 'https://crm-be.local:8011/api',
  // Key PrimeUI (Community miễn phí nếu đủ điều kiện, hoặc Commercial) — cách đăng ký: README §2.
  // Repo public để RỖNG, KHÔNG commit key thật (điều khoản PrimeUI cấm công bố key cho người khác dùng).
  primeuiLicense: '', // dán key PrimeUI CỦA BẠN vào bản copy environment.local.ts (gitignored); rỗng ⇒ banner đỏ "Invalid PrimeUI License"
  // URL file JSON chứa key PrimeUI trên CDN. Repo public để RỖNG ⇒ không fetch CDN. Bản Cluvix đặt trong environment.cluvix.ts (gitignored).
  primeuiLicenseCdnUrl: '',
};
