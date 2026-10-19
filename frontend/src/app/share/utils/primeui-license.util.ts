import { environment } from '@env/environment';

import {
  PRIMEUI_LICENSE_CDN_URL,
  PRIMEUI_LICENSE_CHECK_INTERVAL_MS,
  PRIMEUI_LICENSE_EXPIRY_WARN_DAYS,
  PRIMEUI_LICENSE_FETCH_TIMEOUT_MS,
  STORAGE_KEY_SELECT,
} from '@/define';

/**
 * Nạp key PrimeUI lúc CHẠY thay vì nhúng cứng lúc build.
 *
 * Vì sao cần: key tier "community" hết hạn mỗi 12 tháng. Key nhúng trong bundle nghĩa là
 * mỗi lần gia hạn phải build + deploy lại TOÀN BỘ tenant. Đọc lúc chạy thì gia hạn chỉ còn
 * là thay 1 file JSON trên CDN.
 *
 * Chiến lược "cache-first, refresh nền" — CỐ Ý không fetch trước khi bootstrap:
 *   1. Lúc khởi động (đồng bộ, không mạng): dùng key trong localStorage; chưa có thì dùng
 *      key build-time trong environment.
 *   2. Sau khi app đã chạy: fetch CDN dưới nền, lưu vào cache cho LẦN MỞ SAU.
 *
 * Đánh đổi: key mới có hiệu lực ở lần tải trang kế tiếp, không phải ngay lập tức. Đổi lại
 * CDN chậm/chết KHÔNG bao giờ chặn app khởi động — điều bắt buộc, vì hậu quả của key sai
 * chỉ là một banner đỏ, không đáng để đánh cược thời gian khởi động của 13 tenant production.
 *
 * THỨ TỰ ƯU TIÊN — CDN là nguồn chân lý (user chốt 2026-09-05):
 * cache CHỈ được ghi từ CDN, nên nó chính là "điều CDN nói lần gần nhất"; environment chỉ là
 * lưới đỡ cho máy chưa bao giờ với tới CDN. Cache hợp lệ ⇒ luôn thắng, KHÔNG so `exp`.
 * Từng cân nhắc "chỉ nhận key có exp xa hơn" để chống hạ cấp vô ý, nhưng luật đó chặn luôn
 * việc SỬA SAI: up nhầm key hạn dài rồi muốn quay về key đúng (hạn ngắn hơn) sẽ bị bỏ qua
 * vĩnh viễn, buộc phải build lại 13 tenant — đúng thứ cơ chế này sinh ra để loại bỏ. Đổi lại,
 * key hỏng trên CDN chỉ gây banner đỏ, và key sai hình dạng vẫn bị parseLicense chặn.
 */

/** Payload của license — chỉ khai các field util này thật sự đọc. */
interface PrimeuiLicensePayload {
  product: string;
  exp: number;
}

/** Key đã chốt lúc khởi động. Đọc qua primeuiLicense(), KHÔNG export biến trực tiếp. */
let resolvedLicense = '';

/** Key đang dùng. app.config.ts đọc lazy qua getter nên gọi lúc bootstrap là đã có giá trị. */
export function primeuiLicense(): string {
  return resolvedLicense;
}

/** Giải mã base64url (license dùng base64url, KHÔNG padding — atob không nuốt trực tiếp được). */
function decodeBase64Url(input: string): string | null {
  try {
    const base64 = input.replace(/-/g, '+').replace(/_/g, '/');
    const padded = base64.padEnd(base64.length + ((4 - (base64.length % 4)) % 4), '=');
    return atob(padded);
  } catch {
    return null;
  }
}

/**
 * Trả payload nếu `key` đúng khuôn license PrimeUI, ngược lại null.
 * Chặn việc cache nhầm rác (CDN trả trang lỗi HTML, file rỗng, key sản phẩm khác).
 * KHÔNG kiểm chữ ký — việc đó là của PrimeNG (Ed25519); ở đây chỉ sàng lọc hình dạng.
 */
function parseLicense(key: unknown): PrimeuiLicensePayload | null {
  if (typeof key !== 'string' || !key) return null;

  const parts = key.split('.');
  if (parts.length !== 2 || !parts[0] || !parts[1]) return null;

  const json = decodeBase64Url(parts[0]);
  if (!json) return null;

  try {
    const payload = JSON.parse(json) as Partial<PrimeuiLicensePayload>;
    if (payload?.product !== 'primeui' || typeof payload.exp !== 'number') return null;
    return { product: payload.product, exp: payload.exp };
  } catch {
    return null;
  }
}

/** Số ngày còn lại tới hạn (âm = đã quá hạn). */
function daysUntil(exp: number): number {
  return Math.floor((exp * 1000 - Date.now()) / 86_400_000);
}

/** Đọc key đã cache. Bọc try/catch vì localStorage NÉM lỗi ở Safari private mode. */
function readCachedLicense(): string | null {
  try {
    return localStorage.getItem(STORAGE_KEY_SELECT.PrimeuiLicense);
  } catch {
    return null;
  }
}

function writeCachedLicense(key: string): void {
  try {
    localStorage.setItem(STORAGE_KEY_SELECT.PrimeuiLicense, key);
  } catch {
    // Hết quota / private mode — bỏ qua, lần mở sau lại dùng key build-time.
  }
}

/** Dọn cache hỏng để lần sau không phải parse-fail lại. */
function clearCachedLicense(): void {
  try {
    localStorage.removeItem(STORAGE_KEY_SELECT.PrimeuiLicense);
  } catch {
    // Không xoá được thì thôi — parseLicense vẫn chặn được giá trị hỏng.
  }
}

/** Đã hỏi CDN trong vòng PRIMEUI_LICENSE_CHECK_INTERVAL_MS gần đây chưa. */
function checkedRecently(): boolean {
  try {
    const raw = localStorage.getItem(STORAGE_KEY_SELECT.PrimeuiLicenseCheckedAt);
    if (!raw) return false;
    const last = Number(raw);
    if (!Number.isFinite(last)) return false;
    // Kẹp 2 đầu: đồng hồ máy chạy lùi (last > now) cũng phải cho hỏi lại, không thì
    // kẹt vĩnh viễn tới khi mốc tương lai trôi qua.
    const elapsed = Date.now() - last;
    return elapsed >= 0 && elapsed < PRIMEUI_LICENSE_CHECK_INTERVAL_MS;
  } catch {
    return false;
  }
}

function markChecked(): void {
  try {
    localStorage.setItem(STORAGE_KEY_SELECT.PrimeuiLicenseCheckedAt, String(Date.now()));
  } catch {
    // Không ghi được mốc = lần sau hỏi lại. Chấp nhận được, không phải lỗi.
  }
}

/**
 * Chốt key sẽ dùng cho phiên này. PHẢI gọi TRƯỚC bootstrapApplication (main.ts) vì
 * providePrimeNG đọc `license` ngay trong app initializer lúc bootstrap.
 * Đồng bộ hoàn toàn, không chạm mạng.
 */
export function resolvePrimeuiLicense(): void {
  const cached = readCachedLicense();
  const cachedPayload = parseLicense(cached);

  // Cache hỏng (rác, cắt dở, key sản phẩm khác) → dọn luôn, đừng để parse-fail mỗi lần load.
  if (cached !== null && !cachedPayload) clearCachedLicense();

  // Cache (= bản CDN gần nhất) thắng tuyệt đối; environment chỉ đỡ khi chưa có cache.
  let chosen: { key: string; exp: number } | null = null;
  if (cachedPayload) {
    chosen = { key: cached as string, exp: cachedPayload.exp };
  } else {
    const envPayload = parseLicense(environment.primeuiLicense);
    if (envPayload) chosen = { key: environment.primeuiLicense, exp: envPayload.exp };
  }

  if (!chosen) return;
  resolvedLicense = chosen.key;

  const left = daysUntil(chosen.exp);
  if (left <= PRIMEUI_LICENSE_EXPIRY_WARN_DAYS) {
    const when = new Date(chosen.exp * 1000).toISOString().slice(0, 10);
    console.warn(
      left < 0
        ? `[PrimeUI] License đã hết hạn ${when}. Gia hạn miễn phí tại primeui.dev rồi cập nhật file key trên CDN.`
        : `[PrimeUI] License hết hạn ${when} (còn ${left} ngày). Gia hạn miễn phí tại primeui.dev rồi cập nhật file key trên CDN.`,
    );
  }
}

/**
 * Fetch key mới từ CDN và cache lại cho LẦN MỞ SAU. Gọi sau khi app đã bootstrap.
 * Không bao giờ ném lỗi — mọi trục trặc đều im lặng bỏ qua, app vẫn chạy với key hiện có.
 */
export async function refreshPrimeuiLicenseFromCdn(): Promise<void> {
  if (!PRIMEUI_LICENSE_CDN_URL) return;
  if (checkedRecently()) return;

  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), PRIMEUI_LICENSE_FETCH_TIMEOUT_MS);

  try {
    // 3 tầng cache phải phá, mỗi tầng một cách khác nhau (đã đo trên CDN thật 2026-09-05):
    //  1. Cache HTTP trình duyệt      → `cache: 'no-store'`.
    //  2. Service worker              → loại trừ trong custom-ngsw-worker.js (isLicenseConfig),
    //                                   vì SW chặn request TRƯỚC cache HTTP nên 'no-store' vô tác dụng.
    //  3. CDN edge (X-Cache: HIT)     → query param dưới đây. CDN không trả Cache-Control/ETag/
    //                                   Last-Modified nào ⇒ TTL do edge tự quyết, client không
    //                                   revalidate được. Đo được query string NẰM TRONG cache key
    //                                   (URL trần = HIT, thêm ?d=<mới> = MISS) nên đổi param là
    //                                   ép đi thẳng origin.
    // Chọn mốc NGÀY: key gia hạn 1 lần/năm, nên propagate trong ≤24h là quá đủ, mà vẫn giữ
    // đúng 1 request về origin mỗi ngày thay vì mỗi lần mở app.
    // Mốc ngày theo giờ MÁY (không phải UTC) để "hôm nay" khớp cảm nhận người dùng.
    // Cố ý KHÔNG dùng số ngẫu nhiên: cùng một ngày thì mọi client dùng chung 1 URL, nên
    // edge chỉ phải về origin đúng 1 lần/ngày, phần còn lại phục vụ từ cache của nó.
    const now = new Date();
    const day = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`;
    const url = `${PRIMEUI_LICENSE_CDN_URL}${PRIMEUI_LICENSE_CDN_URL.includes('?') ? '&' : '?'}d=${day}`;

    const res = await fetch(url, { cache: 'no-store', signal: controller.signal });

    // Ghi mốc ngay khi CDN có TRẢ LỜI (kể cả 404/500): đó là câu trả lời dứt khoát của
    // server, hỏi lại mỗi lần mở app cũng vô ích. Ngược lại lỗi mạng/timeout thì rơi xuống
    // catch và KHÔNG ghi mốc — mất mạng là chuyện của client, lần mở sau phải thử lại.
    markChecked();

    if (!res.ok) return;

    const body = (await res.json()) as { primeuiLicense?: unknown };
    const incoming = body?.primeuiLicense;
    // Chỉ cần ĐÚNG HÌNH DẠNG là nhận — KHÔNG so exp với key đang dùng. Xem "THỨ TỰ ƯU TIÊN"
    // ở đầu file: chặn key hạn ngắn hơn sẽ chặn luôn thao tác sửa sai của admin.
    if (!parseLicense(incoming)) return;

    writeCachedLicense(incoming as string);
  } catch {
    // CDN chết, mất mạng, CORS, JSON hỏng, quá timeout — đều không phải lỗi chặn app.
  } finally {
    clearTimeout(timer);
  }
}
