import { HttpErrorResponse } from '@angular/common/http';

import { detailLoadErrorKey } from './detail-load-error';

describe('detailLoadErrorKey', () => {
  it('404 → bản ghi không tồn tại (thử lại vô ích, phải đóng panel)', () => {
    expect(detailLoadErrorKey(new HttpErrorResponse({ status: 404 }))).toBe(
      'common.message.record_not_found',
    );
  });

  // story-14 (epic notification-debt-cleanup): 403 tách khỏi câu chung — đi cùng `SKIP_GLOBAL_403`,
  // panel là nơi DUY NHẤT còn nói được lý do sau khi interceptor thôi điều hướng.
  it('403 → thiếu quyền (thử lại vô ích, khác hẳn "trục trặc tạm thời")', () => {
    expect(detailLoadErrorKey(new HttpErrorResponse({ status: 403 }))).toBe(
      'common.message.no_permission',
    );
  });

  it('lỗi máy chủ → câu chung, thử lại có thể ăn', () => {
    expect(detailLoadErrorKey(new HttpErrorResponse({ status: 500 }))).toBe(
      'common.message.load_failed',
    );
  });

  it('mất mạng (status 0) không được đọc nhầm thành "đã bị xoá"', () => {
    expect(detailLoadErrorKey(new HttpErrorResponse({ status: 0 }))).toBe(
      'common.message.load_failed',
    );
  });

  it('lỗi không phải HttpErrorResponse cũng có câu để hiện', () => {
    expect(detailLoadErrorKey(new Error('boom'))).toBe('common.message.load_failed');
  });
});
