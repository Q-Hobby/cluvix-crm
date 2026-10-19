// src/app/core/handler/global-error-handler.ts
import { ErrorHandler, Service } from '@angular/core';
import { environment } from '@env/environment';

/**
 * Bắt mọi lỗi runtime Angular không ai xử lý.
 *
 * ⚠ Bản trước KHÔNG làm gì trên production: toàn bộ thân hàm nằm trong `if (!environment.prod)` và
 * không có nhánh else. Hậu quả đo được (2026-09-08): một hộp thoại không mở được trên tenant thật,
 * và KHÔNG có gì để lần — không log, không lỗi, không dấu vết. Triệu chứng duy nhất người dùng thấy
 * là "bấm không có gì xảy ra". Một lỗi im lặng đắt hơn nhiều so với một dòng log.
 *
 * Vì vậy: LUÔN in ra console, kể cả production. Console là của chính máy người dùng, không gửi đi
 * đâu, không tốn gì — nhưng nó là thứ duy nhất biến "không hiểu vì sao" thành "đọc dòng lỗi".
 *
 * KHÔNG hiện toast: lỗi tới đây là lỗi lập trình, người dùng cuối không làm gì được với nó; các lỗi
 * cần nói với người dùng (4xx/5xx) đã do `response-handler.interceptor` lo.
 */
@Service()
export class GlobalErrorHandler implements ErrorHandler {
  handleError(error: any): void {
    // Promise rejection được Angular bọc trong `{ rejection }` — bóc ra, nếu không console chỉ in
    // vỏ bọc và mất luôn stack thật.
    const real = error?.rejection ?? error;

    console.error('[Cluvix] Lỗi runtime chưa được xử lý:', real);

    // Chrome tự in stack cho Error object; ca không phải Error (throw string/object) thì không có
    // gì để lần, in thêm stack của vỏ bọc để ít nhất biết nó phát ra từ đâu.
    if (!(real instanceof Error) && error?.stack) {
      console.error('[Cluvix] Stack:', error.stack);
    }

    if (!environment.prod) {
      // Dev: in thêm nguyên vẹn object gốc để soi các field phụ (ngưỡng ồn ào chấp nhận được).
      console.error('[Cluvix] Chi tiết (dev):', error);
    }
  }
}
