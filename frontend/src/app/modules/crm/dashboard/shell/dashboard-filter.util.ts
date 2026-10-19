import { computed, linkedSignal, Signal, untracked, WritableSignal } from '@angular/core';
import type { DashboardQuery } from '@/core/services/crm/crm-dashboard.service';

/**
 * Bộ lọc chung Dashboard CRM (epic crm-dashboard-customizable story-06) — phần THUẦN, không Angular.
 * Hợp đồng BE (`crm/dashboard/handler/filter.go`): `from`/`to` Unix giây, nửa mở [from, to) (`to` LOẠI TRỪ =
 * đầu ngày kế tiếp), gửi CÙNG nhau, ≤ 366 ngày; `branch_id` thuộc phạm vi user.
 *
 * Mốc ngày tính trên "giờ treo tường" của thương hiệu: `nowWall` là Date mà các trường getFullYear/getMonth/getDate
 * mang ngày-giờ THEO MÚI THƯƠNG HIỆU (TimezoneService.now()); `wallToUnix(date)` đổi đầu ngày của date đó ra Unix
 * (TimezoneService.dateHmsToUnix(date, 0, 0, 0)). Không phụ thuộc múi giờ máy chạy.
 */
export type DashboardPreset = 'today' | 'last_7_days' | 'this_month' | 'last_month' | 'this_quarter' | 'this_year' | 'custom';

export const DASHBOARD_PRESETS: readonly Exclude<DashboardPreset, 'custom'>[] = [
  'today', 'last_7_days', 'this_month', 'last_month', 'this_quarter', 'this_year',
];
export const DEFAULT_DASHBOARD_PRESET: DashboardPreset = 'this_month';
/** Khớp BE `define.CrmDashboardRangeMaxDays`. */
export const DASHBOARD_RANGE_MAX_DAYS = 366;

export interface DashboardFilterState {
  preset: DashboardPreset;
  /** Unix giây, chỉ có nghĩa khi preset = custom (preset khác tự tính lúc dùng). */
  from: number | null;
  to: number | null;
  branchId: number | null;
}

export type { DashboardQuery };

export type WallToUnix = (wallDate: Date) => number;
export type UnixToWall = (unix: number) => Date;

const day = (y: number, m: number, d: number) => new Date(y, m, d);

/** [from, to) của 1 preset. */
export function presetRange(preset: Exclude<DashboardPreset, 'custom'>, nowWall: Date, wallToUnix: WallToUnix): { from: number; to: number } {
  const y = nowWall.getFullYear();
  const m = nowWall.getMonth();
  const d = nowWall.getDate();
  switch (preset) {
    case 'today':
      return { from: wallToUnix(day(y, m, d)), to: wallToUnix(day(y, m, d + 1)) };
    case 'last_7_days':
      return { from: wallToUnix(day(y, m, d - 6)), to: wallToUnix(day(y, m, d + 1)) };
    case 'this_month':
      return { from: wallToUnix(day(y, m, 1)), to: wallToUnix(day(y, m + 1, 1)) };
    case 'last_month':
      return { from: wallToUnix(day(y, m - 1, 1)), to: wallToUnix(day(y, m, 1)) };
    case 'this_quarter': {
      const q = Math.floor(m / 3) * 3;
      return { from: wallToUnix(day(y, q, 1)), to: wallToUnix(day(y, q + 3, 1)) };
    }
    case 'this_year':
      return { from: wallToUnix(day(y, 0, 1)), to: wallToUnix(day(y + 1, 0, 1)) };
  }
}

/** Khoảng tuỳ chọn từ 2 ngày (bao gồm) của date picker ⇒ [đầu ngày bắt đầu, đầu ngày SAU ngày kết thúc). */
export function customRange(startWall: Date, endWall: Date, wallToUnix: WallToUnix): { from: number; to: number } {
  const [a, b] = startWall <= endWall ? [startWall, endWall] : [endWall, startWall];
  return {
    from: wallToUnix(day(a.getFullYear(), a.getMonth(), a.getDate())),
    to: wallToUnix(day(b.getFullYear(), b.getMonth(), b.getDate() + 1)),
  };
}

export function isRangeTooLong(from: number, to: number): boolean {
  return to - from > DASHBOARD_RANGE_MAX_DAYS * 86400;
}

/** [from, to) hiệu lực của trạng thái. */
export function effectiveRange(s: DashboardFilterState, nowWall: Date, wallToUnix: WallToUnix): { from: number; to: number } {
  if (s.preset === 'custom' && s.from !== null && s.to !== null) return { from: s.from, to: s.to };
  const p = s.preset === 'custom' ? DEFAULT_DASHBOARD_PRESET : s.preset;
  return presetRange(p as Exclude<DashboardPreset, 'custom'>, nowWall, wallToUnix);
}

/**
 * Tham số gửi BE. Preset mặc định ("Tháng này") KHÔNG gửi from/to ⇒ BE đi đúng đường mặc định cũ (kết quả y như
 * trước epic, cùng khoá cache với mobile). Chi nhánh chỉ gửi khi đã chọn.
 */
export function toApiQuery(s: DashboardFilterState, nowWall: Date, wallToUnix: WallToUnix): DashboardQuery {
  const q: DashboardQuery = {};
  if (s.preset !== DEFAULT_DASHBOARD_PRESET) {
    const r = effectiveRange(s, nowWall, wallToUnix);
    q.from = r.from;
    q.to = r.to;
  }
  if (s.branchId) q.branch_id = s.branchId;
  return q;
}

/** Tên query param trên URL. */
export const URL_PARAM = { preset: 'dr', from: 'from', to: 'to', branch: 'br' } as const;

export function toUrlParams(s: DashboardFilterState): Record<string, string | null> {
  const isDefault = s.preset === DEFAULT_DASHBOARD_PRESET;
  return {
    [URL_PARAM.preset]: isDefault ? null : s.preset,
    [URL_PARAM.from]: s.preset === 'custom' && s.from !== null ? String(s.from) : null,
    [URL_PARAM.to]: s.preset === 'custom' && s.to !== null ? String(s.to) : null,
    [URL_PARAM.branch]: s.branchId ? String(s.branchId) : null,
  };
}

const posInt = (v: string | null | undefined): number | null => {
  if (!v || !/^\d+$/.test(v)) return null;
  const n = Number(v);
  // Số quá lớn (vd 21 chữ số) mất chính xác — chặn ở FE, không để BE trả 422 + toast (QA P8).
  return Number.isSafeInteger(n) && n > 0 ? n : null;
};

/**
 * Đọc trạng thái từ URL (reload / link chia sẻ / Back-Forward). Giá trị rác ⇒ về mặc định — KHÔNG gửi rác lên BE.
 * Chi nhánh ngoài phạm vi user bị bỏ (BE cũng chặn 403, FE bỏ trước để không hiện lỗi vô cớ).
 */
export function parseUrlParams(get: (name: string) => string | null, allowedBranchIds: readonly number[]): DashboardFilterState {
  const raw = get(URL_PARAM.preset);
  let preset: DashboardPreset = DEFAULT_DASHBOARD_PRESET;
  let from: number | null = null;
  let to: number | null = null;
  if (raw === 'custom') {
    from = posInt(get(URL_PARAM.from));
    to = posInt(get(URL_PARAM.to));
    if (from !== null && to !== null && to > from && !isRangeTooLong(from, to)) {
      preset = 'custom';
    } else {
      from = to = null;
    }
  } else if (raw && (DASHBOARD_PRESETS as readonly string[]).includes(raw)) {
    preset = raw as DashboardPreset;
  }
  const br = posInt(get(URL_PARAM.branch));
  const branchId = br !== null && (allowedBranchIds.length === 0 || allowedBranchIds.includes(br)) ? br : null;
  return { preset, from, to, branchId };
}

/**
 * Ngày tham chiếu = đầu ngày (theo thương hiệu) của min(to − 1s, bây giờ) — CÙNG công thức BE `refDayStart`.
 * Trả Unix đầu ngày đó.
 */
export function refDayStart(toExcl: number, nowUnix: number, unixToWall: UnixToWall, wallToUnix: WallToUnix): number {
  const w = unixToWall(Math.min(toExcl - 1, nowUnix));
  return wallToUnix(day(w.getFullYear(), w.getMonth(), w.getDate()));
}

/**
 * Kỳ liền trước dùng cho chip % — CÙNG công thức BE `lastPeriod` (`crm/dashboard/handler/filter.go`): kỳ đúng 1 tháng
 * dương lịch ⇒ tháng trước; khác ⇒ cùng độ dài ngay trước `from`. Chỉ để HIỂN THỊ, số liệu vẫn do BE tính.
 */
export function previousRange(from: number, to: number, unixToWall: UnixToWall, wallToUnix: WallToUnix): { from: number; to: number } {
  const w = unixToWall(from);
  const y = w.getFullYear();
  const m = w.getMonth();
  const startOfDay = w.getHours() === 0 && w.getMinutes() === 0 && w.getSeconds() === 0;
  if (w.getDate() === 1 && startOfDay && wallToUnix(day(y, m + 1, 1)) === to) {
    return { from: wallToUnix(day(y, m - 1, 1)), to: from };
  }
  return { from: from - (to - from), to: from };
}

/**
 * Đổi [from, to) ngược ra 2 ngày (bao gồm) cho date picker. Ngày cuối = ngày chứa `to − 1s` — KHÔNG `to − 86400`:
 * ở múi có DST ngày cuối có thể dài 23h/25h ⇒ lệch 1 ngày (QA story-06 P7).
 */
export function datesFromRange(from: number, to: number, unixToWall: UnixToWall): [Date, Date] {
  return [unixToWall(from), unixToWall(to - 1)];
}

/** Unix nửa đêm KẾ TIẾP theo giờ thương hiệu — tính từ ngày treo tường +1 (không `+86400`: ngày DST dài 23h/25h). */
export function nextMidnight(nowWall: Date, wallToUnix: WallToUnix): number {
  return wallToUnix(day(nowWall.getFullYear(), nowWall.getMonth(), nowWall.getDate() + 1));
}

export function sameQuery(a: DashboardQuery, b: DashboardQuery): boolean {
  return a.from === b.from && a.to === b.to && a.branch_id === b.branch_id;
}

/**
 * Khoá KỲ của trạng thái (preset|from|to) — CỐ Ý không gồm chi nhánh: nguồn của linkedSignal ở thanh lọc, đổi chi
 * nhánh lúc đang chọn dở khoảng ngày không được xoá nháp (QA story-06 P10).
 */
export function periodKeyOf(s: DashboardFilterState): string {
  return `${s.preset}|${s.from ?? ''}|${s.to ?? ''}`;
}

/**
 * 2 trạng thái giao diện của ô "Tuỳ chọn" ở thanh lọc, BÁM theo KỲ thật (Back/Forward) nhưng KHÔNG reset khi chỉ đổi
 * chi nhánh (QA story-06 P10/P13). `computation` của linkedSignal VẪN bị theo dõi (Angular 22 chạy nó khi node là
 * consumer đang hoạt động) ⇒ đọc `state` trong đó phải bọc `untracked`, phụ thuộc DUY NHẤT là `source` = khoá kỳ.
 */
export function createCustomPickerState(
  state: Signal<DashboardFilterState>,
  toDates: (s: DashboardFilterState) => Date[] | null,
): { open: WritableSignal<boolean>; draft: WritableSignal<Date[] | null> } {
  const periodKey = computed(() => periodKeyOf(state()));
  const open = linkedSignal<string, boolean>({
    source: periodKey,
    computation: (key) => key.startsWith('custom|'),
  });
  const draft = linkedSignal<string, Date[] | null>({
    source: periodKey,
    computation: () => untracked(() => toDates(state())),
  });
  return { open, draft };
}
