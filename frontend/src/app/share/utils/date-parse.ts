export function parseDateInput(raw: string): Date | null {
  if (!raw) return null;
  // Chấp nhận "dd/MM/yyyy" hoặc "dd/MM/yyyy HH:mm"
  const [datePart, timePart] = raw.trim().split(/\s+/);
  const [d, m, y] = (datePart || '').split(/[\/\-]/).map(Number);
  if (!d || !m || !y) return null;

  let hh = 0,
    mm = 0;
  if (timePart) {
    const t = timePart.split(':').map(Number);
    hh = t[0] ?? 0;
    mm = t[1] ?? 0;
  }
  const dt = new Date(y, m - 1, d, hh, mm, 0, 0);
  return isNaN(dt.getTime()) ? null : dt;
}
