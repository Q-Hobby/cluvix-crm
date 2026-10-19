import { environment } from '../../environments/environment';

export const TAG_SEVERITY_OPTION = [
  { value: 'success', label: 'Success', labelKey: 'common.status.success' },
  { value: 'secondary', label: 'Secondary', labelKey: 'define.tag_severity.secondary' },
  { value: 'info', label: 'Info', labelKey: 'common.label.info' },
  { value: 'warn', label: 'Warn', labelKey: 'common.label.warning' },
  { value: 'danger', label: 'Danger', labelKey: 'define.tag_severity.danger' },
  { value: 'contrast', label: 'Contrast', labelKey: 'define.tag_severity.contrast' },
];

// ─── PrimeUI license ────────────────────────────────────────────────────────
// Key tier "community" hết hạn sau 12 tháng (grace 30 ngày), gia hạn MIỄN PHÍ hằng năm.
// Trước đây key nhúng cứng lúc build → mỗi lần gia hạn phải build + deploy lại toàn bộ
// tenant. Nay key đọc lúc CHẠY từ 1 file JSON trên CDN; giá trị trong environment chỉ còn
// là fallback khi CDN chưa cấu hình/không với tới được.
// Quá hạn KHÔNG làm hỏng chức năng — PrimeNG chỉ hiện banner đỏ "Invalid PrimeUI License".

/**
 * URL file JSON chứa key trên CDN. Rỗng = tắt hẳn việc fetch, chỉ dùng key build-time.
 * ⚠ Đổi TÊN FILE ở đây ⇒ phải sửa cả `LICENSE_FILE` trong src/custom-ngsw-worker.js
 * (service worker phải cho request này đi thẳng mạng; 2 file không import được nhau).
 */
// Repo public (user chốt 2026-10-19): để RỖNG ⇒ không fetch CDN, chỉ dùng key build-time
// `primeuiLicense` (hoặc hiện banner). Bản Cluvix đặt `primeuiLicenseCdnUrl` trong
// src/environments/environment.cluvix.ts (gitignored) và build bằng `npm run build:cluvix`.
// ⚠ Giữ kiểu `string` tường minh: literal `''` sẽ làm TS suy ra type `never` sau guard
// `if (!PRIMEUI_LICENSE_CDN_URL) return;` ở primeui-license.util.ts, hỏng build production.
export const PRIMEUI_LICENSE_CDN_URL: string = environment.primeuiLicenseCdnUrl || '';

/**
 * Giãn cách tối thiểu giữa 2 lần hỏi CDN (ms). Key đổi 1 lần/năm nên hỏi mỗi lần mở app là
 * lãng phí: 1 request cross-origin thừa × mọi user × 13 tenant. 24h vừa khớp độ mịn của
 * cache-buster theo ngày ở primeui-license.util.ts nên không mất gì về độ tươi.
 */
export const PRIMEUI_LICENSE_CHECK_INTERVAL_MS = 86_400_000;

/** Timeout fetch CDN (ms). Fetch chạy NỀN sau bootstrap nên không chặn khởi động app. */
export const PRIMEUI_LICENSE_FETCH_TIMEOUT_MS = 8000;

/** Còn dưới ngần này ngày là hết hạn → console.warn nhắc gia hạn. */
export const PRIMEUI_LICENSE_EXPIRY_WARN_DAYS = 45;
