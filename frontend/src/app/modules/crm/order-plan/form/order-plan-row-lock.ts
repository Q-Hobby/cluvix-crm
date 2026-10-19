/**
 * Quy tắc khoá TỪNG Ô của 1 dòng dịch vụ trong `OrderPlanForm` — tách ra HÀM THUẦN để kiểm được
 * bằng spec, không cần trình duyệt (story-08, sau khi smoke bắt lỗi khoá hụt).
 *
 * ⚠ BÀI HỌC GỐC (2026-09-21): KHÔNG khoá ô bằng `[readonly]`/`[disabled]` trên template khi control
 * đó có `[formField]`. Directive `FormField` của Signal Forms ĐẨY NGƯỢC trạng thái field xuống chính
 * các input `readonly`/`disabled` của component (`signals.mjs`: `FIELD_STATE_KEY_TO_CONTROL_BINDING`
 * + `host.setInputOnDirectives(name, value)`), nên giá trị template gán bị GHI ĐÈ bằng trạng thái
 * field (mặc định `false`) — ô vẫn gõ được, mà `tsc`/`ngc` đều xanh. Nơi DUY NHẤT khoá thật là
 * schema (`disabled()` trong `serviceRowSchema`), đọc cờ khoá ghi trên chính dòng đó.
 */

export type LockableRowField = 'unit_price' | 'service_id';

/** Cờ khoá FE-only nằm trên chính dòng dịch vụ (`OrderServiceUsedRow`). */
export interface RowLockFlags {
  /** Khoá riêng ô Đơn giá (combo lock / min==max / giá ngoài khoảng). */
  unitPriceLocked?: boolean | null;
  /** Khoá riêng ô Dịch vụ (dòng đã có payment / đang/đã bán). */
  serviceLocked?: boolean | null;
}

/** 1 ô của dòng có bị khoá không — chỉ `unit_price` và `service_id` có thể bị khoá, mọi ô khác tự do. */
export function isRowFieldLocked(field: LockableRowField, flags: RowLockFlags | null | undefined): boolean {
  if (field === 'unit_price') return !!flags?.unitPriceLocked;
  if (field === 'service_id') return !!flags?.serviceLocked;
  return false;
}
