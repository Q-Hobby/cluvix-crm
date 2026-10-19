import { describe, expect, it, vi } from 'vitest';

/**
 * ⚠ Giả lập môi trường PRODUCTION cho cả file này. Đây là điểm mấu chốt, không phải chi tiết:
 * bản lỗi cũ nuốt sạch lỗi CHỈ trên production (`if (!environment.prod)` không có nhánh else), nên
 * một test chạy ở môi trường dev sẽ XANH với cả bản hỏng lẫn bản đúng — lưới vô dụng đúng lúc cần.
 * Đã đo: cấy lại bản cũ mà không mock, test vẫn 2/2 pass.
 */
vi.mock('@env/environment', () => ({ environment: { prod: true } }));

import { GlobalErrorHandler } from './global-error-handler';

/**
 * Lưới chống TÁI PHÁT: `handleError` phải THẬT SỰ báo lỗi ra ngoài. Im lặng là hỏng.
 *
 * Sự cố gốc (2026-09-08): hộp thoại không mở được trên tenant thật, không log, không dấu vết — chỉ
 * có "bấm không thấy gì". Không tra được nguyên nhân vì handler này không nói gì cả.
 */
describe('GlobalErrorHandler — TRÊN PRODUCTION cũng không được nuốt lỗi', () => {
  it('lỗi thường: in ra console.error kèm chính đối tượng lỗi', () => {
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {});
    try {
      const err = new Error('bùm');
      new GlobalErrorHandler().handleError(err);
      expect(spy).toHaveBeenCalled();
      expect(spy.mock.calls.some((c) => c.includes(err))).toBe(true);
    } finally {
      spy.mockRestore();
    }
  });

  it('promise rejection: bóc `rejection` ra thay vì in vỏ bọc', () => {
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {});
    try {
      const real = new Error('rejection thật');
      new GlobalErrorHandler().handleError({ rejection: real });
      expect(spy.mock.calls.some((c) => c.includes(real))).toBe(true);
    } finally {
      spy.mockRestore();
    }
  });

  it('throw chuỗi (không phải Error): vẫn phải báo ra, không im lặng', () => {
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {});
    try {
      new GlobalErrorHandler().handleError('lỗi dạng chuỗi');
      expect(spy).toHaveBeenCalled();
    } finally {
      spy.mockRestore();
    }
  });
});
