/**
 * Helper tuổi cho bản in bệnh án.
 *
 * Năm sinh ưu tiên lấy từ `patient.birthday` (ngày sinh đầy đủ), fallback
 * `patient.year_born` (chỉ năm). `birthday` BE trả dạng chuỗi ISO
 * (vd `1990-04-12T00:00:00Z`) → parse qua Date.
 */
export function birthYearOf(patient: any): number | null {
  if (!patient) return null;
  if (patient.birthday) {
    const y = new Date(patient.birthday).getFullYear();
    if (!Number.isNaN(y)) return y;
  }
  if (patient.year_born) {
    const y = +patient.year_born;
    if (!Number.isNaN(y) && y > 0) return y;
  }
  return null;
}

/**
 * Tuổi dạng 3 chữ số cho ô vuông trên mẫu in (vd 28 → ['0','2','8']).
 * Trả 3 ô rỗng khi không có năm sinh.
 */
export function ageDigitsOf(patient: any, currentYear: number): string[] {
  const by = birthYearOf(patient);
  if (by == null) return ['', '', ''];
  const age = Math.max(0, currentYear - by);
  return age.toString().padStart(3, '0').split('');
}
