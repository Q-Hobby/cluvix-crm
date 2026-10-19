// Learn more about Vitest configuration options at https://vitest.dev/config/

import { readFileSync } from 'node:fs';
import { dirname, resolve as resolvePath } from 'node:path';
import { fileURLToPath } from 'node:url';
import { defineConfig, type Plugin } from 'vitest/config';

const src = (p: string) => fileURLToPath(new URL(p, import.meta.url));

/**
 * Nội tuyến `templateUrl` và gỡ `styleUrl(s)` trong decorator `@Component` trước khi esbuild dịch TS.
 *
 * Vì sao cần: vitest chạy Angular ở chế độ JIT. JIT KHÔNG tự nạp được file `.html`/`.scss` cạnh
 * component ⇒ `TestBed.configureTestingModule({ imports: [SomeComponent] })` ném
 * "Component 'X' is not resolved: - templateUrl: ./x.html — Did you run and wait for
 * resolveComponentResources()?". Angular CLI (`ng test`) không dính vì builder của nó biên dịch AOT.
 *
 * ⚠ Đây là phép biến đổi VĂN BẢN, chỉ chạy trong test — KHÔNG ảnh hưởng bundle sản phẩm.
 * Nếu một ngày có component viết `templateUrl` bằng biến (không phải chuỗi hằng) thì plugin này
 * KHÔNG khớp và spec sẽ đỏ với đúng thông báo "is not resolved" ở trên — đỏ chứ không xanh giả.
 */
function inlineAngularResources(): Plugin {
  const TEMPLATE_URL = /templateUrl\s*:\s*(['"])([^'"]+)\1\s*,?/g;
  const STYLE_URLS = /styleUrls?\s*:\s*(\[[^\]]*\]|(['"])[^'"]*\2)\s*,?/g;

  return {
    name: 'cluvix-inline-angular-resources',
    enforce: 'pre',
    transform(code, id) {
      const file = id.split('?')[0];
      if (!file.endsWith('.ts') || file.includes('/node_modules/')) return null;
      if (!code.includes('templateUrl') && !/styleUrls?\s*:/.test(code)) return null;

      const dir = dirname(file);
      let out = code.replace(TEMPLATE_URL, (_m, _q, rel: string) => {
        const html = readFileSync(resolvePath(dir, rel), 'utf8');
        return `template: ${JSON.stringify(html)},`;
      });
      out = out.replace(STYLE_URLS, 'styles: [],');
      return out === code ? null : { code: out, map: null };
    },
  };
}

export default defineConfig({
  plugins: [inlineAngularResources()],
  // Alias phải KHỚP `compilerOptions.paths` trong `tsconfig.json`. Vitest KHÔNG tự đọc tsconfig paths:
  // thiếu khai ở đây thì mọi spec chạm một file dùng `@/...` chết ngay ở bước import với
  // "Cannot find package '@/...'" — cả FILE spec đỏ, `Tests: no tests`, nên nhìn thoáng dễ tưởng
  // spec chưa viết chứ không phải hạ tầng test hỏng.
  //
  // Đã xảy ra: `container/omnichat.ts` bắt đầu import `@/share/services/teamchat.service` và toàn bộ
  // spec omnichat tắt tiếng cùng lúc. Thêm path mới vào tsconfig ⇒ thêm cả vào đây.
  resolve: {
    alias: [
      { find: /^@env\/(.*)$/, replacement: src('./src/environments/$1') },
      { find: /^@assets\/(.*)$/, replacement: src('./src/assets/$1') },
      { find: /^@\/(.*)$/, replacement: src('./src/app/$1') },
    ],
  },
  test: {
    // Bật globals cho khớp `types: ["vitest/globals"]` ở `tsconfig.spec.json`: khai type mà không bật
    // ở runtime thì biên dịch qua nhưng chạy là `describe is not defined` — đỏ ở tầng khác, cùng một
    // nguyên nhân. Hai chỗ này là MỘT quyết định, đừng sửa lẻ một bên.
    globals: true,

    // jsdom: bắt buộc. Không có DOM thì (a) spec nào chạm `document` chết với
    // "document is not defined", (b) `TestBed.createComponent` không dựng được component nào.
    environment: 'jsdom',

    // Khởi tạo TestBed một lần cho cả suite — xem `src/test-setup.ts`.
    setupFiles: [src('./src/test-setup.ts')],
  },
});
