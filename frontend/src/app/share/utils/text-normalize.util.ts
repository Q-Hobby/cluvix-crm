const COMBINING_DIACRITICS_RE = /[̀-ͯ]/g;

/** Lowercase + bo dau tieng Viet - dung cho search/filter client-side tren danh muc lon. */
export function normalizeSearchText(value: string | null | undefined): string {
  return (value || '')
    .normalize('NFD')
    .replace(COMBINING_DIACRITICS_RE, '')
    .replace(/đ/g, 'd')
    .replace(/Đ/g, 'D')
    .toLowerCase();
}
