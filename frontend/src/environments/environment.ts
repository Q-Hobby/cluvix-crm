export const environment = {
  prod: false,
  option: 3,
  apiUrl: 'https://localhost:8011/api',
  // PrimeUI license (community, miễn phí, gia hạn hằng năm) — mặc định rỗng, KHÔNG commit key
  // thật vào repo public. Hoặc đặt primeuiLicenseCdnUrl (field bên dưới) nếu tự
  // host CDN riêng, hoặc dán trực tiếp giá trị ở dòng dưới trên bản build LOCAL của bạn.
  primeuiLicense: '', // self-host: dán license PrimeUI community của bạn ở đây (xem README) hoặc để rỗng — chỉ hiện banner đỏ, không hỏng chức năng
  // URL file JSON chứa key PrimeUI trên CDN. Repo public để RỖNG ⇒ không fetch CDN. Bản Cluvix đặt trong environment.cluvix.ts (gitignored).
  primeuiLicenseCdnUrl: '',
};
