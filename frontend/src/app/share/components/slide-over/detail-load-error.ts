import { HttpErrorResponse } from '@angular/common/http';

/**
 * Chọn câu báo lỗi cho panel chi tiết khi nạp dữ liệu hỏng.
 *
 * Tách 404 khỏi các lỗi còn lại vì hai thứ đó người dùng phải xử lý khác nhau: 404 = bản ghi đã bị
 * xoá (bấm "Thử lại" bao nhiêu lần cũng vậy, phải đóng panel và làm mới danh sách), lỗi khác = trục
 * trặc tạm thời, thử lại thường ăn. Trả về KHOÁ i18n để template dịch bằng `| translate`.
 *
 * Đi kèm `SKIP_GLOBAL_404` (response-handler.interceptor): request nào dùng khoá này thì phải tự
 * hiển thị lỗi, vì interceptor đã thôi điều hướng sang `/auth/notfound`.
 */
export function detailLoadErrorKey(err: unknown): string {
  if (!(err instanceof HttpErrorResponse)) return 'common.message.load_failed';
  // story-14 (epic notification-debt-cleanup): 403 tách khỏi "trục trặc tạm thời" vì "Thử lại" không
  // bao giờ ăn — phải nói rõ là thiếu quyền. Đi kèm `SKIP_GLOBAL_403`: request nào đặt cờ đó thì
  // interceptor đã thôi điều hướng sang `/auth/accessdenied`, panel phải tự hiển thị lý do.
  if (err.status === 403) return 'common.message.no_permission';
  return err.status === 404 ? 'common.message.record_not_found' : 'common.message.load_failed';
}
