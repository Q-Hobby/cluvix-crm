import { signal } from '@angular/core';
import { describe, expect, it } from 'vitest';

import {
  createCustomPickerState, customRange, datesFromRange, effectiveRange, isRangeTooLong, nextMidnight, parseUrlParams, presetRange, previousRange, refDayStart,
  periodKeyOf, toApiQuery, toUrlParams, type DashboardFilterState,
} from './dashboard-filter.util';

// Công ty ở UTC+7 (ICT), không phụ thuộc múi máy: dùng các trường y/m/d của Date làm "giờ treo tường".
const OFFSET = 7 * 3600;
const wallToUnix = (d: Date) => Date.UTC(d.getFullYear(), d.getMonth(), d.getDate(), d.getHours(), d.getMinutes(), d.getSeconds()) / 1000 - OFFSET;
const unixToWall = (u: number) => {
  const x = new Date((u + OFFSET) * 1000);
  return new Date(x.getUTCFullYear(), x.getUTCMonth(), x.getUTCDate(), x.getUTCHours(), x.getUTCMinutes(), x.getUTCSeconds());
};
const ict = (y: number, m: number, d: number) => Date.UTC(y, m - 1, d) / 1000 - OFFSET;
// "Bây giờ" = 08/10/2026 15:30 giờ công ty.
const NOW = new Date(2026, 9, 8, 15, 30);
const NOW_UNIX = wallToUnix(NOW);

describe('presetRange — [from, to) theo giờ công ty, to LOẠI TRỪ', () => {
  it.each([
    ['today', ict(2026, 10, 8), ict(2026, 10, 9)],
    ['last_7_days', ict(2026, 10, 2), ict(2026, 10, 9)],
    ['this_month', ict(2026, 10, 1), ict(2026, 11, 1)],
    ['last_month', ict(2026, 9, 1), ict(2026, 10, 1)],
    ['this_quarter', ict(2026, 10, 1), ict(2027, 1, 1)],
    ['this_year', ict(2026, 1, 1), ict(2027, 1, 1)],
  ] as const)('%s', (p, from, to) => {
    expect(presetRange(p, NOW, wallToUnix)).toEqual({ from, to });
  });
  it('tháng 1 ⇒ tháng trước là 12 năm trước; quý 1', () => {
    const jan = new Date(2027, 0, 15);
    expect(presetRange('last_month', jan, wallToUnix)).toEqual({ from: ict(2026, 12, 1), to: ict(2027, 1, 1) });
    expect(presetRange('this_quarter', jan, wallToUnix)).toEqual({ from: ict(2027, 1, 1), to: ict(2027, 4, 1) });
  });
});

describe('customRange / trần 366 ngày', () => {
  it('2 ngày bao gồm ⇒ to = đầu ngày sau ngày cuối; đảo thứ tự vẫn đúng', () => {
    const r = customRange(new Date(2026, 8, 1), new Date(2026, 8, 30), wallToUnix);
    expect(r).toEqual({ from: ict(2026, 9, 1), to: ict(2026, 10, 1) });
    expect(customRange(new Date(2026, 8, 30), new Date(2026, 8, 1), wallToUnix)).toEqual(r);
  });
  it('366 ngày hợp lệ, 367 quá dài', () => {
    expect(isRangeTooLong(0, 366 * 86400)).toBe(false);
    expect(isRangeTooLong(0, 367 * 86400)).toBe(true);
  });
});

describe('toApiQuery', () => {
  const base: DashboardFilterState = { preset: 'this_month', from: null, to: null, branchId: null };
  it('mặc định ⇒ KHÔNG gửi tham số nào (BE đi đường mặc định cũ)', () => {
    expect(toApiQuery(base, NOW, wallToUnix)).toEqual({});
  });
  it('mặc định + chi nhánh ⇒ chỉ branch_id', () => {
    expect(toApiQuery({ ...base, branchId: 3 }, NOW, wallToUnix)).toEqual({ branch_id: 3 });
  });
  it('preset khác ⇒ from/to; custom ⇒ đúng from/to đã chọn', () => {
    expect(toApiQuery({ ...base, preset: 'last_month' }, NOW, wallToUnix)).toEqual({ from: ict(2026, 9, 1), to: ict(2026, 10, 1) });
    expect(toApiQuery({ ...base, preset: 'custom', from: 100, to: 200 }, NOW, wallToUnix)).toEqual({ from: 100, to: 200 });
  });
  it('custom thiếu mốc ⇒ coi như mặc định', () => {
    expect(effectiveRange({ ...base, preset: 'custom' }, NOW, wallToUnix)).toEqual(presetRange('this_month', NOW, wallToUnix));
  });
});

describe('URL', () => {
  const map = (o: Record<string, string>) => (k: string) => o[k] ?? null;
  it('mặc định ⇒ không có tham số; round-trip giữ nguyên', () => {
    const s: DashboardFilterState = { preset: 'custom', from: ict(2026, 9, 1), to: ict(2026, 10, 1), branchId: 2 };
    const p = toUrlParams(s);
    expect(toUrlParams({ preset: 'this_month', from: null, to: null, branchId: null })).toEqual({ dr: null, from: null, to: null, br: null });
    expect(parseUrlParams((k) => p[k] ?? null, [1, 2])).toEqual(s);
  });
  it('rác ⇒ mặc định; custom quá 366 ngày / đảo ngược ⇒ mặc định; chi nhánh ngoài phạm vi ⇒ bỏ', () => {
    expect(parseUrlParams(map({ dr: 'hack' }), []).preset).toBe('this_month');
    expect(parseUrlParams(map({ dr: 'custom', from: '100', to: String(100 + 367 * 86400) }), []).preset).toBe('this_month');
    expect(parseUrlParams(map({ dr: 'custom', from: '200', to: '100' }), []).preset).toBe('this_month');
    expect(parseUrlParams(map({ dr: 'custom', from: '5abc', to: '100' }), []).preset).toBe('this_month');
    expect(parseUrlParams(map({ br: '9' }), [1, 2]).branchId).toBeNull();
    expect(parseUrlParams(map({ br: '2' }), [1, 2]).branchId).toBe(2);
    expect(parseUrlParams(map({ dr: 'last_7_days' }), []).preset).toBe('last_7_days');
    // Số vượt Number.MAX_SAFE_INTEGER (21 chữ số) ⇒ mất chính xác ⇒ bỏ (QA P8).
    expect(parseUrlParams(map({ dr: 'custom', from: '123456789012345678901', to: '123456789012345678999' }), []).preset).toBe('this_month');
    expect(parseUrlParams(map({ br: '99999999999999999999' }), []).branchId).toBeNull();
  });
});

describe('refDayStart — cùng công thức BE', () => {
  it('kỳ chứa hôm nay ⇒ hôm nay', () => {
    expect(refDayStart(ict(2026, 11, 1), NOW_UNIX, unixToWall, wallToUnix)).toBe(ict(2026, 10, 8));
  });
  it('tháng trước ⇒ ngày cuối tháng trước', () => {
    expect(refDayStart(ict(2026, 10, 1), NOW_UNIX, unixToWall, wallToUnix)).toBe(ict(2026, 9, 30));
  });
  it('kỳ tương lai ⇒ hôm nay', () => {
    expect(refDayStart(ict(2027, 1, 1), NOW_UNIX, unixToWall, wallToUnix)).toBe(ict(2026, 10, 8));
  });
});

// Múi CÓ DST (America/New_York) — đổi qua lại bằng Intl thật, không phụ thuộc múi máy.
const NY = 'America/New_York';
function nyOffsetSec(unix: number): number {
  const p = new Intl.DateTimeFormat('en-US', { timeZone: NY, hour12: false, year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' })
    .formatToParts(new Date(unix * 1000));
  const g = (t: string) => Number(p.find((x) => x.type === t)!.value) % (t === 'hour' ? 24 : 100000);
  return (Date.UTC(g('year'), g('month') - 1, g('day'), g('hour'), g('minute'), g('second')) / 1000) - unix;
}
const nyWallToUnix = (d: Date) => {
  const guess = Date.UTC(d.getFullYear(), d.getMonth(), d.getDate(), d.getHours(), d.getMinutes(), d.getSeconds()) / 1000;
  let u = guess - nyOffsetSec(guess);
  u = guess - nyOffsetSec(u);
  return u;
};
const nyUnixToWall = (u: number) => {
  const x = new Date((u + nyOffsetSec(u)) * 1000);
  return new Date(x.getUTCFullYear(), x.getUTCMonth(), x.getUTCDate(), x.getUTCHours(), x.getUTCMinutes(), x.getUTCSeconds());
};

describe('previousRange — cùng công thức BE lastPeriod', () => {
  it('đúng 1 tháng dương lịch ⇒ cả tháng trước (tháng 10 có 31 ngày ⇒ tháng 9 có 30)', () => {
    expect(previousRange(ict(2026, 10, 1), ict(2026, 11, 1), unixToWall, wallToUnix)).toEqual({ from: ict(2026, 9, 1), to: ict(2026, 10, 1) });
  });
  it('tháng 1 ⇒ tháng 12 năm trước', () => {
    expect(previousRange(ict(2027, 1, 1), ict(2027, 2, 1), unixToWall, wallToUnix)).toEqual({ from: ict(2026, 12, 1), to: ict(2027, 1, 1) });
  });
  it.each([
    ['7 ngày', ict(2026, 10, 2), ict(2026, 10, 9), ict(2026, 9, 25)],
    ['hôm nay', ict(2026, 10, 8), ict(2026, 10, 9), ict(2026, 10, 7)],
    ['quý 1 (90 ngày) ⇒ 90 ngày trước, KHÔNG phải trọn quý 4', ict(2027, 1, 1), ict(2027, 4, 1), ict(2026, 10, 3)],
    ['tuỳ chọn bắt đầu ngày 1 nhưng không trọn tháng', ict(2026, 10, 1), ict(2026, 10, 16), ict(2026, 9, 16)],
  ] as const)('%s ⇒ cùng độ dài ngay trước from', (_n, from, to, want) => {
    expect(previousRange(from, to, unixToWall, wallToUnix)).toEqual({ from: want, to: from });
  });
});

describe('DST — ngày 23h/25h (QA P7, P11)', () => {
  it('datesFromRange: ngày cuối = ngày chứa to−1 (ngày 08/03/2026 chỉ dài 23h ở New York)', () => {
    const from = nyWallToUnix(new Date(2026, 2, 1));
    const to = nyWallToUnix(new Date(2026, 2, 9)); // [01/03, 09/03) ⇒ ngày cuối 08/03
    const [a, b] = datesFromRange(from, to, nyUnixToWall);
    expect([a.getDate(), a.getMonth()]).toEqual([1, 2]);
    expect([b.getDate(), b.getMonth()]).toEqual([8, 2]);
  });
  it('nextMidnight theo NGÀY treo tường, không +86400 (01/11/2026 dài 25h ở New York)', () => {
    const now = new Date(2026, 10, 1, 20, 0, 0);
    const want = nyWallToUnix(new Date(2026, 10, 2));
    expect(nextMidnight(now, nyWallToUnix)).toBe(want);
    expect(want - nyWallToUnix(new Date(2026, 10, 1))).toBe(25 * 3600);
  });
  it('nextMidnight ngày thường (UTC+7)', () => {
    expect(nextMidnight(NOW, wallToUnix)).toBe(ict(2026, 10, 9));
  });
});

describe('periodKeyOf — nguồn linkedSignal thanh lọc (QA P10)', () => {
  it('đổi chi nhánh KHÔNG đổi khoá kỳ; đổi preset/from/to thì đổi', () => {
    const a: DashboardFilterState = { preset: 'custom', from: 1, to: 2, branchId: null };
    expect(periodKeyOf({ ...a, branchId: 5 })).toBe(periodKeyOf(a));
    expect(periodKeyOf({ ...a, preset: 'today' })).not.toBe(periodKeyOf(a));
    expect(periodKeyOf({ ...a, from: 9 })).not.toBe(periodKeyOf(a));
    expect(periodKeyOf({ ...a, to: 9 })).not.toBe(periodKeyOf(a));
  });
});

describe('createCustomPickerState — nối thật vào thanh lọc (QA P13)', () => {
  const toDates = (st: DashboardFilterState) => (st.preset === 'custom' && st.from !== null ? [new Date(st.from), new Date(st.to!)] : null);

  it('đang mở "Tuỳ chọn" + chọn dở ⇒ đổi CHI NHÁNH vẫn giữ mở và giữ nháp', () => {
    const st = signal<DashboardFilterState>({ preset: 'this_month', from: null, to: null, branchId: null });
    const { open, draft } = createCustomPickerState(st, toDates);
    expect(open()).toBe(false);
    open.set(true);
    const half = [new Date(2026, 8, 1)];
    draft.set(half);
    st.set({ ...st(), branchId: 7 });
    expect(open()).toBe(true);
    expect(draft()).toBe(half);
  });

  it('đổi KỲ từ ngoài (Back/Forward) ⇒ reset theo kỳ thật', () => {
    const st = signal<DashboardFilterState>({ preset: 'this_month', from: null, to: null, branchId: null });
    const { open, draft } = createCustomPickerState(st, toDates);
    open.set(true);
    draft.set([new Date(2026, 8, 1)]);
    st.set({ preset: 'today', from: null, to: null, branchId: null });
    expect(open()).toBe(false);
    expect(draft()).toBeNull();
    st.set({ preset: 'custom', from: 1000, to: 2000, branchId: null });
    expect(open()).toBe(true);
    expect(draft()).toEqual([new Date(1000), new Date(2000)]);
  });
});
