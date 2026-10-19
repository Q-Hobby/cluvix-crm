import DOMPurify from 'dompurify';

/**
 * Allowlist tag — bám theo HTML mà hệ thống THẬT SỰ sinh ra:
 * - `p-editor` (Quill) toolbar mặc định của PrimeNG: header, bold, italic, underline, color,
 *   background, font, align, list, link, image, code-block → p/br/strong/em/u/span/h1-6/ol/ul/li/a/img/pre.
 * - Nội dung cũ import từ hệ thống v19 và tin nhắn omnichat từ provider: bảng, gạch ngang, chỉ số
 *   trên/dưới, đường kẻ ngang.
 *
 * KHÔNG thêm `iframe`/`video`/`audio`/`object`/`embed`/`form`/`input`/`svg`: đó chính là các vector
 * XSS/clickjacking, và không toolbar nào trong repo sinh ra chúng. Muốn nhúng video → làm component
 * riêng, đừng nới allowlist này.
 *
 * ⚠ Tag/attr thiếu ở đây bị strip **câm lặng** — không lỗi, không cảnh báo. Thêm định dạng mới cho
 * editor thì phải cập nhật danh sách này cùng lúc.
 */
const ALLOWED_TAGS = [
  // khối văn bản
  'p', 'br', 'div', 'span', 'blockquote', 'pre', 'code', 'hr',
  'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
  // định dạng chữ
  'strong', 'b', 'em', 'i', 'u', 's', 'strike', 'del', 'ins', 'sub', 'sup', 'small', 'mark', 'abbr',
  // danh sách
  'ol', 'ul', 'li', 'dl', 'dt', 'dd',
  // liên kết & ảnh
  'a', 'img', 'figure', 'figcaption',
  // bảng
  'table', 'caption', 'colgroup', 'col', 'thead', 'tbody', 'tfoot', 'tr', 'td', 'th',
];

/**
 * `loading`/`decoding`: `LazyImgPipe` chèn vào `<img>` trước khi qua pipe này — thiếu ở đây là mất
 * lazy-load ở list view mà không ai biết.
 * `colspan`/`rowspan`: bảng trong mẫu in và nội dung cũ. Thuộc tính `data-*` được DOMPurify giữ sẵn
 * (ALLOW_DATA_ATTR mặc định true) nên `data-checked` của checklist Quill vẫn sống.
 * `style`: DOMPurify **KHÔNG** lọc giá trị CSS (đã đo: `style="background:url(javascript:alert(1))"`
 * đi qua nguyên vẹn) → phải tự lọc bằng hook bên dưới.
 */
const ALLOWED_ATTR = [
  'href', 'target', 'rel', 'src', 'alt', 'title', 'class', 'style', 'dir',
  'width', 'height', 'loading', 'decoding',
  'colspan', 'rowspan', 'align', 'valign', 'span', 'start',
];

/**
 * Thuộc tính CSS được phép trong `style`. Bám theo thứ Quill sinh ra (màu chữ/nền, canh lề, cỡ chữ,
 * thụt đầu dòng) + bảng của nội dung cũ. KHÔNG có `position`/`z-index`/`opacity`: nội dung do người
 * dùng nhập mà đặt được lớp phủ toàn màn hình là mở đường clickjacking.
 */
const SAFE_CSS_PROPS = new Set([
  'color', 'background', 'background-color',
  'text-align', 'text-decoration', 'text-indent', 'white-space',
  'font-size', 'font-family', 'font-weight', 'font-style', 'line-height', 'direction',
  'padding', 'padding-left', 'padding-right', 'padding-top', 'padding-bottom',
  'margin', 'margin-left', 'margin-right', 'margin-top', 'margin-bottom',
  'width', 'height', 'max-width', 'vertical-align',
  'border', 'border-color', 'border-width', 'border-style', 'border-collapse',
]);

/** Cú pháp CSS có thể nạp tài nguyên/thực thi — chặn bất kể thuộc tính nào. */
const DANGEROUS_CSS = /url\s*\(|expression\s*\(|javascript:|behaviou?r\s*:|-moz-binding|@import/i;

/** Giữ lại đúng các khai báo CSS nằm trong allowlist và không chứa cú pháp nguy hiểm. */
function sanitizeStyleValue(value: string): string {
  return value
    .split(';')
    .map((declaration) => declaration.trim())
    .filter((declaration) => {
      const separator = declaration.indexOf(':');
      if (separator < 0) return false;
      const prop = declaration.slice(0, separator).trim().toLowerCase();
      const val = declaration.slice(separator + 1);
      return SAFE_CSS_PROPS.has(prop) && !DANGEROUS_CSS.test(val);
    })
    .join('; ');
}

// Hook đăng ký MỘT lần cho cả app (DOMPurify là singleton) — mọi lời gọi `DOMPurify.sanitize` khác
// trong repo cũng được hưởng lớp lọc CSS này, không nơi nào yếu đi.
let styleHookRegistered = false;
function registerStyleHook(): void {
  if (styleHookRegistered) return;
  styleHookRegistered = true;
  DOMPurify.addHook('uponSanitizeAttribute', (_node, data) => {
    if (data.attrName !== 'style') return;
    data.attrValue = sanitizeStyleValue(data.attrValue);
    if (!data.attrValue) data.keepAttr = false;
  });
}

/**
 * Làm sạch HTML người dùng nhập. Tách khỏi pipe để test thuần được (không cần TestBed) — allowlist ở
 * đây là lớp chặn XSS duy nhất nên phải có lưới test đi kèm: `safehtml.pipe.spec.ts`.
 */
export function sanitizeUserHtml(value: unknown): string {
  registerStyleHook();
  return DOMPurify.sanitize((value ?? '') as string, { ALLOWED_TAGS, ALLOWED_ATTR });
}
