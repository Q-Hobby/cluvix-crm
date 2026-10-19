import { TimezoneService } from '@/core/services/timezone.service';
import {
  FilterFieldDef,
  MAX_TEXT_FILTER_LENGTH,
  emptyValueOf,
  fromDateOnlyString,
  fromMonthDayString,
  isBlankFilterValue,
  toDateOnlyString,
  toMonthDayString,
  urlKeyOf,
} from '@/share/base/list-filter.model';

/**
 * Encode/decode trạng thái list (filter + phân trang + sort) ra query param URL.
 *
 * Định dạng: param phẳng, đọc được bằng mắt, không có matchMode trong URL
 * (matchMode suy ra từ FilterFieldDef lúc parse):
 *
 *   /crm/appointment?ap_f=1&ap_page=2&ap_sort=-appointment_date
 *                   &ap_appointment_date=1754006400,1754092799
 *                   &ap_status=1,2&ap_keyword=nguyen
 *
 * Prefix (`ap_`) để 2 list nằm chung 1 route không giẫm param của nhau.
 *
 * `<prefix>_f=1` là cờ đánh dấu "URL đang giữ trạng thái". Có cờ → URL là nguồn sự thật,
 * param filter vắng mặt nghĩa là NGƯỜI DÙNG ĐÃ XOÁ filter đó, không phải quay về mặc định.
 * Thiếu cờ (vào trang từ menu) → dùng giá trị mặc định khai trong FilterFieldDef.
 */

/**
 * Phần giao nhau giữa `ParamMap` (Angular) và `URLSearchParams` (DOM) — đủ để đọc state.
 * Cần vì `NavigationSkipped` không cập nhật snapshot route, lúc đó phải parse từ URL đích.
 */
export interface ListUrlParams {
  has(key: string): boolean;
  get(key: string): string | null;
}

/** Hậu tố các param dùng riêng cho state, không phải tên filter */
export const URL_STATE_FLAG = 'f';
export const URL_STATE_PAGE = 'page';
export const URL_STATE_SIZE = 'size';
export const URL_STATE_SORT = 'sort';

/** Tên param đầy đủ của 1 filter */
export function filterParamName(prefix: string, def: FilterFieldDef): string {
  return `${prefix}_${urlKeyOf(def)}`;
}

/** Tên param đầy đủ của 1 khoá state */
export function stateParamName(prefix: string, key: string): string {
  return `${prefix}_${key}`;
}

/** URL hiện tại có đang giữ state của list này không */
export function hasListUrlState(prefix: string, map: ListUrlParams): boolean {
  return !!prefix && map.has(stateParamName(prefix, URL_STATE_FLAG));
}

/**
 * Giá trị control → chuỗi param. Trả null khi không lọc (caller ghi null để router
 * xoá hẳn key khỏi URL thay vì để lại `key=`).
 */
export function serializeFilterValue(
  def: FilterFieldDef,
  value: any,
  tz: TimezoneService,
): string | null {
  if (isBlankFilterValue(value)) return null;

  if (def.kind === 'dateRange') {
    const range = tz.buildFilterTimestamp(value);
    return range ? `${range[0]},${range[1]}` : null;
  }

  // Ngày đơn: đưa lên URL đúng chuỗi `yyyy-MM-dd` mà BE nhận, không qua unix.
  if (def.kind === 'dateSingle') {
    return value instanceof Date ? toDateOnlyString(value) : String(value);
  }

  // Khoảng ngày-tháng không năm: URL ra `MM-DD,MM-DD`.
  // PHẢI đặt TRƯỚC nhánh Array.isArray bên dưới, nếu không cặp [Date, Date] rơi vào nhánh generic
  // và ra chuỗi `Mon Aug 03 2026...` rác trên URL.
  // Range đang chọn dở (p-datePicker trả [Date, null]) → null: chưa ghi lên URL.
  if (def.kind === 'monthDayRange') {
    const [a, b] = Array.isArray(value) ? value : [value, value];
    if (!(a instanceof Date) || !(b instanceof Date)) return null;
    return `${toMonthDayString(a)},${toMonthDayString(b)}`;
  }

  // Khoảng ngày gửi BE bằng 2 param tên riêng: URL ra `yyyy-MM-dd,yyyy-MM-dd`.
  // Cũng PHẢI đặt trước nhánh Array.isArray (cùng lý do như monthDayRange ở trên).
  if (def.kind === 'dateRangeSplit') {
    const [a, b] = Array.isArray(value) ? value : [value, value];
    if (!(a instanceof Date) || !(b instanceof Date)) return null;
    return `${toDateOnlyString(a)},${toDateOnlyString(b)}`;
  }

  if (Array.isArray(value)) {
    const items = value.filter((v) => v !== null && v !== undefined && v !== '');
    return items.length ? items.join(',') : null;
  }

  const str = String(value).trim();
  return str.length ? str : null;
}

/** Chuỗi param → giá trị control (ngược của serializeFilterValue) */
export function parseFilterValue(def: FilterFieldDef, raw: string, tz: TimezoneService): any {
  if (raw === null || raw === undefined || raw === '') return emptyValueOf(def);

  if (def.kind === 'dateRange') {
    const [from, to] = raw.split(',');
    const start = tz.fromUnix(from);
    const end = tz.fromUnix(to ?? from);
    return start && end ? [start, end] : emptyValueOf(def);
  }

  if (def.kind === 'dateSingle') return fromDateOnlyString(raw) ?? emptyValueOf(def);

  // Ngược của nhánh monthDayRange ở serializeFilterValue: `MM-DD,MM-DD` → [Date, Date]
  // (p-datePicker selectionMode="range" cần Date[], không nhận chuỗi).
  // Hỏng 1 đầu → coi như không lọc, không trả range nửa vời.
  if (def.kind === 'monthDayRange') {
    const [from, to] = raw.split(',');
    const start = fromMonthDayString(from);
    const end = fromMonthDayString(to ?? from);
    return start && end ? [start, end] : emptyValueOf(def);
  }

  // Ngược của nhánh dateRangeSplit ở serializeFilterValue: `yyyy-MM-dd,yyyy-MM-dd` → [Date, Date].
  // Đây chính là đường parse ngược mà `kind: 'custom'` không có — lý do filter loại này
  // trước đây phải để `urlSync: false` và không sống sót qua Back/F5.
  if (def.kind === 'dateRangeSplit') {
    const [from, to] = raw.split(',');
    const start = fromDateOnlyString(from);
    const end = fromDateOnlyString(to ?? from);
    return start && end ? [start, end] : emptyValueOf(def);
  }

  // Chặn trên độ dài — link chia sẻ/khôi phục URL không được bơm lại giá trị vượt ngưỡng
  // vào query string GET (xem lý do ở khai báo `MAX_TEXT_FILTER_LENGTH`).
  if (def.kind === 'text') return raw.slice(0, MAX_TEXT_FILTER_LENGTH);

  const asItem = (s: string) => (def.itemType === 'string' ? s : coerceNumber(s));

  // 'in' (multiselect) LUÔN là mảng; 'equals' (select 1 giá trị) LUÔN là scalar.
  // Trước đây 'equals' cũng tách mảng khi URL có dấu phẩy → control single nhận vào mảng và
  // hành xử như multiselect. Phân biệt phải nằm ở `kind`, không phải ở hình dạng chuỗi URL.
  if (def.kind === 'in') return raw.split(',').map(asItem);
  if (def.kind === 'custom') return raw.includes(',') ? raw.split(',').map(asItem) : asItem(raw);
  return asItem(raw.split(',')[0]);
}

/** '12' → 12, '0912xxx' giữ nguyên chuỗi (số 0 đứng đầu là dữ liệu, không phải số) */
function coerceNumber(s: string): number | string {
  return /^-?(0|[1-9]\d*)$/.test(s) ? Number(s) : s;
}
