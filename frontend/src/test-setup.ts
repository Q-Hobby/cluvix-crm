/**
 * Setup dùng chung cho vitest chạy TRỰC TIẾP (`npx vitest run --config vitest-base.config.ts`).
 *
 * Vì sao cần: `TestBed` của Angular KHÔNG tự khởi tạo. Thiếu file này thì mọi spec gọi
 * `TestBed.configureTestingModule(...)` chết ngay với "Need to call TestBed.initTestEnvironment()
 * first" — đỏ ở tầng hạ tầng, dễ bị đọc nhầm thành "component hỏng".
 *
 * ⛔ KHÔNG đổi tên/di chuyển file mà không sửa `setupFiles` trong `vitest-base.config.ts`.
 *
 * `initTestEnvironment` NÉM LỖI nếu được gọi lần hai. Angular CLI (`ng test`, builder
 * `@angular/build:unit-test`) tự khởi tạo môi trường test của nó — nếu một ngày builder đó cũng nạp
 * file này thì lần gọi thứ hai phải im lặng bỏ qua, KHÔNG được làm sập cả suite. Đó là lý do có
 * try/catch dưới đây (không phải nuốt lỗi bừa).
 */
import 'zone.js';
import 'zone.js/testing';

import { getTestBed } from '@angular/core/testing';

import { BrowserTestingModule, platformBrowserTesting } from '@angular/platform-browser/testing';

try {
  getTestBed().initTestEnvironment(BrowserTestingModule, platformBrowserTesting(), {
    errorOnUnknownElements: false,
    errorOnUnknownProperties: false,
  });
} catch (e) {
  // Đã khởi tạo trước đó (runner khác đã gọi) ⇒ bỏ qua. Mọi lỗi KHÁC phải nổi lên.
  if (!String((e as Error)?.message ?? e).includes('already')) {
    throw e;
  }
}

/**
 * jsdom KHÔNG cài `indexedDB`. Nhiều service của app (`BaseIndexedDBService` → master-data registry)
 * mở IDB ngay trong constructor theo kiểu fire-and-forget; thiếu `indexedDB` thì mỗi lần dựng
 * component là một `ReferenceError` + Unhandled Rejection — vitest coi đó là lỗi của FILE spec và
 * báo đỏ cả file, che mất lỗi thật.
 *
 * Stub dưới đây trả về một `IDBOpenDBRequest` KHÔNG BAO GIỜ phát event: promise mở DB nằm chờ
 * vĩnh viễn ⇒ không resolve, không reject, không rác. Đúng ngữ nghĩa "máy này không có IndexedDB,
 * dữ liệu master không tới" — mà mọi spec hiện có đều không phụ thuộc.
 *
 * ⛔ Spec nào THẬT SỰ cần đọc/ghi IndexedDB thì KHÔNG dùng stub này được (sẽ treo, không đỏ) —
 * phải tự cài `fake-indexeddb` cho riêng spec đó và ghi rõ lý do.
 */
if (typeof globalThis.indexedDB === 'undefined') {
  const neverSettles = () => ({
    result: null,
    error: null,
    onerror: null,
    onsuccess: null,
    onblocked: null,
    onupgradeneeded: null,
    addEventListener: () => {},
    removeEventListener: () => {},
  });
  Object.defineProperty(globalThis, 'indexedDB', {
    configurable: true,
    value: {
      open: neverSettles,
      deleteDatabase: neverSettles,
      databases: async () => [],
      cmp: () => 0,
    },
  });
}
