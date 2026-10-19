/** BE `calcTrend`: 0 = không đổi, 1 = giảm, 2 = tăng. */
export type TrendDirection = 0 | 1 | 2;
export type TrendTone = 'good' | 'bad' | 'neutral';

/** Số đầy đủ theo locale (thẻ KPI hiện tường minh, KHÔNG viết gọn "tr"/"tỷ" — user chốt 2026-10-09). */
export function fullNumber(value: number | string | null | undefined, locale: string): string {
  const n = typeof value === 'string' ? Number(value) : value;
  if (n === null || n === undefined || !Number.isFinite(n)) return '—';
  return new Intl.NumberFormat(locale).format(n);
}

/** % thay đổi (1 chữ số thập phân) theo locale. */
export function percentNumber(value: number | string | null | undefined, locale: string): string {
  const n = typeof value === 'string' ? Number(value) : value;
  if (n === null || n === undefined || !Number.isFinite(n)) return '—';
  return new Intl.NumberFormat(locale, { maximumFractionDigits: 1 }).format(n);
}

/**
 * Màu chip Δ: tăng là tốt (mặc định) ⇒ xanh; `upIsGood=false` (vd hoàn tiền, khách mất) ⇒ tăng là đỏ.
 * Không đổi / không có kỳ trước ⇒ xám.
 */
export function trendTone(direction: TrendDirection | null | undefined, upIsGood = true): TrendTone {
  if (direction === 2) return upIsGood ? 'good' : 'bad';
  if (direction === 1) return upIsGood ? 'bad' : 'good';
  return 'neutral';
}
