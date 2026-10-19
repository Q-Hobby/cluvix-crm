// Single source of truth cho màu chart — đọc từ CSS variables của brand.
// Fallback hex dùng khi SSR hoặc CSS chưa load.
const css = typeof document !== 'undefined'
  ? getComputedStyle(document.documentElement)
  : null;

const primary = css?.getPropertyValue('--cluvix-primary').trim() || '#7B3F9E';
const accent  = css?.getPropertyValue('--cluvix-accent').trim()  || '#FF5722';
const danger  = css?.getPropertyValue('--cluvix-danger').trim()  || '#E53935';

export const CHART_COLORS = { primary, accent, danger };

// Palette mặc định cho multi-series chart (theo thứ tự ưu tiên)
export const DEFAULT_CHART_COLORS: string[] = [
  primary,
  accent,
  danger,
  '#009688', // teal
  '#E91E63', // pink
];
