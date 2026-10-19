import { HttpContextToken } from '@angular/common/http';

/**
 * Override labelKey thủ công cho 1 request cụ thể — dùng khi URL dùng CHUNG giữa nhiều nghiệp vụ
 * (vd `/user-profile/update-user-profile` vừa là "Cập nhật hồ sơ" vừa là "Đổi ngôn ngữ") nên
 * `resolveLoadingLabelKey` suy theo URL không phân biệt được. Set qua `ApiOptions.context` (cùng
 * pattern với `DEDUPE_DISABLE` ở `api-dedupe.interceptor.ts`); `loadingInterceptor` ưu tiên giá trị
 * này trước khi suy theo method/url.
 */
export const LOADING_LABEL_OVERRIDE = new HttpContextToken<string | undefined>(() => undefined);

/**
 * Suy luận i18n label key cho lượt loading của một HTTP request non-GET.
 *
 * Pure function — không side-effect, không inject gì (dễ test độc lập và để
 * `loading.interceptor.ts` gọi trực tiếp mà không cần DI thêm). Chỉ TRẢ VỀ string
 * key; việc dịch (translate) do `AppLoader` đảm nhiệm ở tầng UI.
 *
 * Thứ tự match: cụ thể → chung (theo architecture.md). Rule 5/6/7 (theo method)
 * bao phủ gần như mọi request non-GET; rule 8 chỉ là fallback thật sự cho method
 * lạ (vd OPTIONS/HEAD).
 */
export function resolveLoadingLabelKey(method: string, url: string): string {
  const pathOnly = url.split('?')[0];

  // 1. Đăng nhập (kể cả bước 2FA)
  if (method === 'POST' && /\/auth\/login(\/2fa)?$/.test(pathOnly)) {
    return 'auth.login.submitting';
  }
  // 2. Đăng xuất
  if (method === 'POST' && /\/auth\/logout$/.test(pathOnly)) {
    return 'common.label.logging_out';
  }
  // 3. Import — PHẢI đứng trước rule export: `/import-stock-export` là luồng IMPORT dù tên có
  // chữ "export". Khớp theo ranh giới `/import` + `-` `/` hoặc hết chuỗi để không dính `/important`.
  // Không có rule này thì mọi import rơi vào rule 5 (POST generic) và loader ghi "Đang tạo mới".
  if (/\/import(-|\/|$)/.test(pathOnly)) {
    return 'common.label.importing';
  }
  // 4. Export — cả endpoint riêng (`/export-data-excel`, `/export-fhir`, `/export-xml`,
  // `/export-by-id`) lẫn quy ước dùng chung `?_export=1` trên chính endpoint list.
  if (/\/export(-|\/|$)/.test(pathOnly) || /[?&]_export=1(&|$)/.test(url)) {
    return 'common.label.exporting';
  }
  // 4.5. Đổi thương hiệu (switch company/branch) — URL riêng, đứng trước rule POST generic để
  // không rơi vào "Đang tạo mới". Đổi ngôn ngữ dùng chung URL `update-user-profile` với sửa hồ sơ
  // nên KHÔNG suy được ở đây — component tự set qua `LOADING_LABEL_OVERRIDE` (xem switch-language.ts).
  if (method === 'POST' && /\/user-profile\/switch-profile$/.test(pathOnly)) {
    return 'common.label.switching';
  }
  // 5. Tạo mới (generic theo method)
  if (method === 'POST') {
    return 'common.label.creating';
  }
  // 6. Cập nhật (generic theo method)
  if (method === 'PUT' || method === 'PATCH') {
    return 'common.label.updating';
  }
  // 7. Xoá (generic theo method)
  if (method === 'DELETE') {
    return 'common.label.deleting';
  }
  // 8. Fallback thật sự (method lạ, vd OPTIONS/HEAD)
  return 'common.label.loading';
}

/**
 * Suy luận i18n label key từ KEY của lượt `GlobalLoadingService.showLoading(key)` do component tự gọi.
 *
 * Vì sao cần: export theo quy ước `?_export=1` là request **GET**, mà `loadingInterceptor` chỉ gắn
 * label cho non-GET (GET chỉ chạy thanh progress). Overlay lúc đó do chính component bật bằng
 * `showLoading('exportXxx')` — và cả 153 lời gọi trong `modules/` đều KHÔNG truyền labelKey, nên
 * loader rơi về "Đang tải...". Suy từ key giúp 46 endpoint export GET hiển thị đúng mà không phải
 * sửa từng callsite.
 *
 * Trả `undefined` khi không suy được — caller giữ nguyên hành vi cũ.
 */
export function resolveLoadingLabelKeyFromKey(key: string): string | undefined {
  const k = key.toLowerCase();
  // import trước export: key kiểu `importStockExport` phải ra "đang import".
  if (k.includes('import')) return 'common.label.importing';
  if (k.includes('export')) return 'common.label.exporting';
  return undefined;
}
