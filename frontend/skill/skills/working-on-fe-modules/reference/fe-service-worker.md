# Custom Service Worker

## Mục lục

- Navigate handler — CRITICAL gotcha
- File download bị SW chặn — CRITICAL gotcha
- Double registration: `provideServiceWorker` + manual register
- SWR background revalidate: bắt buộc `event.waitUntil()`
- `respondWith` không được nhận `null`/`undefined` — CRITICAL gotcha
- `response.clone()` phải gọi trước mọi `await`
- `trimCache()`: dùng `cache.keys().length`, không dùng in-memory Map size
- Config JSON cross-origin trên S3 bị SW nuốt — `cache: 'no-store'` KHÔNG cứu
- `beforeinstallprompt` (cài PWA) phải bắt trong `index.html`, không trong service Angular
- Cache strategy tổng quan

---


File: `src/custom-ngsw-worker.js`
Registration: `main.ts` (manual, production only)

---

## Navigate handler — CRITICAL gotcha

**`fetch(req)` vs `fetch('/index.html')`:**

- `fetch(req)` trong SW navigate handler đổi `Sec-Fetch-Mode` từ `navigate` → `same-origin`
- Server middleware có thể redirect deep SPA route (`/crm/xxx`) → `/auth/accessdenied` khi thấy non-navigate mode
- Angular router đọc URL từ `window.location`, không từ response URL → serve `/index.html` cho mọi SPA route là đúng
- **FIX:** luôn `fetch('/index.html')` thay vì `fetch(req)`

```js
// ✅ ĐÚNG
const response = await fetch('/index.html');

// ❌ SAI — strips Sec-Fetch-Mode: navigate
const response = await fetch(req);
```

---

## File download bị SW chặn — CRITICAL gotcha

- `window.location.href = '/download/export/file.xlsx'` tạo navigate request → SW intercept → serve `index.html` thay vì file
- Phải check non-SPA path **trước** navigate handler, return sớm không gọi `respondWith`
- **Iframe load cũng là `mode === 'navigate'`**: trang `.html` tĩnh phục vụ cùng domain app (vd `/widget.html` của
  livechat) phải vào `NON_SPA_EXACT = ['/widget.html']`, không thì SW trả `index.html` → Angular router hiện "trang không
  tồn tại" trong khung chat. Chỉ tái hiện với trình duyệt ĐÃ cài SW (đã từng đăng nhập app) nên dev/khách mới không thấy
  (2026-09-04). Thêm trang tĩnh mới dưới domain app = thêm vào danh sách này.

```js
const NON_SPA_PREFIX = ['/download/'];
const NON_SPA_EXT = /\.(xlsx|xls|csv|pdf|zip|doc|docx|rar|7z)$/i;
const isNonSpaNavigate = (pathname) =>
  NON_SPA_PREFIX.some((p) => pathname.startsWith(p)) || NON_SPA_EXT.test(pathname);

// Trong fetch handler, TRƯỚC navigate handler:
if (req.mode === 'navigate' && isNonSpaNavigate(url.pathname)) return;
```

BE routes phục vụ file download:
- `/download/export` → `./storage/export` (export xlsx)
- `/download/templates` → `./storage/templates`

---

## Double registration: `provideServiceWorker` + manual register

- Nếu cả `provideServiceWorker` (app.config.ts) lẫn `navigator.serviceWorker.register()` (main.ts) cùng tồn tại → double registration
- `provideServiceWorker` inject `SwUpdate`/`SwPush` service — chỉ hoạt động khi SW implement ngsw protocol
- Custom SW không implement ngsw → `SwUpdate.isEnabled = false`, `SwPush` không dùng được
- **Rule:** chỉ dùng 1 trong 2 — manual register nếu không cần `SwUpdate`; `provideServiceWorker` nếu cần

---

## SWR background revalidate: bắt buộc `event.waitUntil()`

- Khi trả cache stale, network fetch chạy background → SW có thể bị killed trước khi fetch xong → cache không được update
- Phải gọi `fetchEvent.waitUntil(networkPromise)` để gia hạn SW lifetime
- Truyền `fetchEvent` vào hàm SWR:

```js
// Trong fetch handler:
event.respondWith(staleWhileRevalidate(req, event));

// Trong hàm SWR:
async function staleWhileRevalidate(request, fetchEvent) {
  // ...
  if (cached && networkPromise) {
    fetchEvent.waitUntil(networkPromise); // ← gia hạn SW lifetime
    return cached;
  }
}
```

---

## `respondWith` không được nhận `null`/`undefined` — CRITICAL gotcha

- Fallback của SWR phải trả **`Response` thật**. `event.respondWith()` nhận giá trị falsy → trình duyệt ném TypeError ra console và request thành network error khó lần ngược về SW.
- `return networkPromise || fallback` **KHÔNG** bảo vệ được: `||` kiểm chính đối tượng Promise (luôn truthy), không kiểm giá trị nó resolve ra. Phải `await` trước rồi mới `||`, hoặc đặt fallback ngay trong `.catch()`.
- Tiền lệ: `staleWhileRevalidate` trong `custom-ngsw-worker.js` từng dính đúng lỗi này — `.catch(() => null)` + `return networkPromise || …` khiến nhánh fallback 504 thành nhánh chết. Đã vá bằng `await` trước khi fallback (2026-09-05).

```js
// ❌ SAI — networkPromise là Promise, luôn truthy; resolve ra null vẫn lọt
return networkPromise || new Response(null, { status: 504 });

// ✅ ĐÚNG
const res = await networkPromise;
return res || new Response(null, { status: 504 });
```

---

## `response.clone()` phải gọi trước mọi `await`

- Trong fetch handler, `clone()` phải chạy **ngay khi response về, trước bất kỳ `await` nào**. Trễ tới sau `await caches.open()` là trình duyệt đã bắt đầu đọc body bản gốc (bản vừa trả cho trang qua `respondWith`) → `Failed to execute 'clone' on 'Response': Response body is already used`, và asset không bao giờ vào cache.
- Ghi cache là việc nền → bọc `event.waitUntil()`, đừng fire-and-forget: SW bị kill giữa chừng là mất luôn lượt ghi.

```js
// ❌ SAI — clone() chạy ở microtask sau, body gốc đã bị đọc
fetch(req).then((res) => {
  caches.open(CACHE).then((c) => c.put(req, res.clone()));
  return res;
});

// ✅ ĐÚNG
const res = await fetch(req);
const copy = res.clone();                    // clone NGAY
event.waitUntil((async () => {
  const c = await caches.open(CACHE);
  await c.put(req, copy);
})());
return res;
```

---

## `trimCache()`: dùng `cache.keys().length`, không dùng in-memory Map size

- In-memory Map (`assetTouched`) reset về 0 khi SW restart → guard `Map.size <= MAX` luôn true sau restart → trim không bao giờ chạy
- Phải đọc `cache.keys()` thực tế để đếm

```js
// ✅ ĐÚNG
async function trimCache(cache) {
  const keys = await cache.keys();
  if (keys.length <= MAX_CACHE_ENTRIES) return;
  const toDelete = keys.length - MAX_CACHE_ENTRIES;
  await Promise.all(keys.slice(0, toDelete).map((req) => cache.delete(req)));
}

// ❌ SAI — in-memory, reset sau restart
if (assetTouched.size <= MAX_CACHE_ENTRIES) return;
```

- Gọi `trimCache(cache)` từ **cả 2 path**: hashed asset (`cacheFirstImmutable`) lẫn SWR (`staleWhileRevalidate`)
- Truyền `cache` object vào hàm thay vì mở lại bên trong

---

## Config JSON cross-origin trên S3 bị SW nuốt — `cache: 'no-store'` KHÔNG cứu

`isS3Url()` cache MỌI path trên `S3_ORIGIN (CDN nội bộ)` trừ `.jpg/.jpeg/.webp` ⇒ file JSON cấu hình đọc lúc runtime cũng rơi vào SWR: stale tới 5 phút, và chiếm slot `ASSET_CACHE` (giới hạn 300 entry, dùng chung với ảnh/js).

- SW chặn request **TRƯỚC** cache HTTP ⇒ `fetch(url, { cache: 'no-store' })` KHÔNG bypass được SW.
- Config runtime phải loại trừ tường minh trong `fetch` handler (network-only), đặt cạnh rule `/api/*`.
- Khớp theo **TÊN FILE**, không theo path — đổi thư mục trên CDN mà quên sửa SW sẽ hỏng âm thầm.

---

## `beforeinstallprompt` (cài PWA) phải bắt trong `index.html`, không trong service Angular

- Chrome/Edge bắn `beforeinstallprompt` MỘT lần/lần tải trang, RẤT SỚM — trước khi Angular bootstrap xong (APP_INITIALIZER) ⇒ listener đặt trong service/component lỡ mất, không lỗi gì, nút cài không bao giờ hiện.
- Cách làm: `<script>` inline trong `src/index.html` bắt + `preventDefault()` + gán `window.__cluvixInstallPrompt`; `PwaInstallService` (`core/services/pwa-install.service.ts`) đọc lại + nghe event `cluvix-installprompt` cho lần bắn sau.
- Verify phải reload và dùng event THẬT (`!!window.__cluvixInstallPrompt`) — dispatch event giả SAU bootstrap luôn xanh, che đúng lỗi timing.

---

## Cache strategy tổng quan

| Request type | Strategy | Cache bucket |
|---|---|---|
| `/api/*` | Network-only | — |
| Hashed chunk `chunk-ABCD1234.js` | Cache-first (immutable) | `ASSET_CACHE` (v1, persist qua deploy) |
| Static assets, i18n JSON | Stale-While-Revalidate TTL 5 min | `ASSET_CACHE` |
| S3 non-heavy | SWR | `ASSET_CACHE` |
| SPA navigate | `fetch('/index.html')` + cache | `STATIC_CACHE` (versioned per SW) |
| `/download/*`, `.xlsx/.pdf/...` | Pass-through | — |
