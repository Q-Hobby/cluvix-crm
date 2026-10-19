/**
 * Dòng "Ngày … tháng … năm …" cho khối ký trên bản in bệnh án.
 * - Đã ký số (`signed_at` unix giây): điền ngày ký thật → "Ngày 16 tháng 06 năm 2026".
 * - Chưa ký: giữ dấu chấm để ký tay điền.
 */
export function signedDateLabel(signedAt: number | null | undefined): string {
  if (!signedAt) return 'Ngày …… tháng …… năm ………';
  const dt = new Date(signedAt * 1000);
  if (Number.isNaN(dt.getTime())) return 'Ngày …… tháng …… năm ………';
  const p = (n: number) => String(n).padStart(2, '0');
  return `Ngày ${p(dt.getDate())} tháng ${p(dt.getMonth() + 1)} năm ${dt.getFullYear()}`;
}
