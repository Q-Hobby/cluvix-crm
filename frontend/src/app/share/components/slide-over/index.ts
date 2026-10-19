import { SlideOver } from './slide-over';
import { SlideOverRow } from './slide-over-row';
import { SlideOverSection } from './slide-over-section';
import { SlideOverActionsDirective, SlideOverFooterDirective } from './slide-over-slots';

export * from './slide-over';
export * from './slide-over-row';
export * from './slide-over-section';
export * from './slide-over-slots';
export * from './detail-load-error';

/** Import gọn cho component dùng SlideOver: `imports: [...SLIDE_OVER_IMPORTS]`. */
export const SLIDE_OVER_IMPORTS = [
  SlideOver,
  SlideOverSection,
  SlideOverRow,
  SlideOverActionsDirective,
  SlideOverFooterDirective,
] as const;
