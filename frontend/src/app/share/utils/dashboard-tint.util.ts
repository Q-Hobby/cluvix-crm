/**
 * Helper màu dark-safe cho dashboard (KPI/stat tile).
 *
 * Thay cho hardcode `bg-{color}-50` / `border-{color}-200` (không flip theo
 * theme) bằng `color-mix()` trộn màu brand với CSS var nền/chữ của PrimeNG —
 * tự sáng/tối theo `.app-dark`. Xem `common/ui.md` (palette) + `frontend.md` §B.18.
 *
 * Chuẩn tham chiếu: care/dashboard/component/kpi-cards.ts.
 */

/** Nền/viền tint theo % màu brand trên nền content (dark-safe). */
export function tint(color: string, percent: number): string {
  return `color-mix(in srgb, ${color} ${percent}%, var(--p-content-background))`;
}

/**
 * Màu icon/chữ nhấn theo nhóm, đạt contrast ở CẢ 2 theme: trộn 45% màu nhóm với màu chữ theme (sáng ⇒ đậm lại,
 * tối ⇒ sáng lên). `shade()` 85% không đủ cho màu sáng như Amber trên nền tint.
 */
export function ink(color: string): string {
  return `color-mix(in srgb, ${color} 45%, var(--p-text-color))`;
}

/** Text đậm theo màu brand nhưng vẫn đọc được ở cả 2 theme. */
export function shade(color: string): string {
  return `color-mix(in srgb, ${color} 85%, var(--p-text-color))`;
}
