// Mẫu environment cho bản build prod CÓ KEY (Cluvix và self-host đều dùng). Copy thành environment.cluvix.ts
// (gitignored) rồi điền primeuiLicense = key PrimeUI của bạn (README §2.4). primeuiLicenseCdnUrl: tuỳ chọn.
// Build: SW_S3_ORIGIN=https://<your-cdn> npm run build:cluvix
export const environment = {
  prod: true,
  option: 3,
  apiUrl: '/api',
  primeuiLicense: '',
  // (1) URL file JSON key PrimeUI trên CDN của bạn — điền để app nạp key lúc chạy.
  // Không tự host CDN key ⇒ để ''.
  primeuiLicenseCdnUrl: 'https://<your-cdn>/static/primeuiLicense/primeui-license.json',
  // (2) Service worker KHÔNG đọc được file này: origin CDN/S3 (khớp S3_CDN của backend/.env)
  //     đặt qua biến môi trường SW_S3_ORIGIN lúc `npm run build:cluvix` (xem scripts/inject-sw-origin.js).
};
