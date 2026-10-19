/** Chỉ cho redirect path nội bộ — chống open-redirect (//evil.com, javascript:, http://...). */
export function safeInternalRedirect(value: string | null): string | null {
  if (!value) return null;
  if (!value.startsWith('/') || value.startsWith('//')) return null;
  return value;
}
