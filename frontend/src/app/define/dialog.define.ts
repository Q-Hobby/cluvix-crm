// Breakpoint responsive dùng chung cho mọi dialog mở qua DialogWrapperService — width khai báo
// theo px cố định (baseline theo nội dung dialog), breakpoints chỉ SHRINK khi màn hình NHỎ hơn
// ngưỡng, KHÔNG scale rộng ra theo % trên màn lớn (tránh dialog phình to bất thường trên ultra-wide).
export const DIALOG_RESPONSIVE_BREAKPOINTS: Record<string, string> = {
  '960px': '90vw',
  '640px': '95vw',
};
