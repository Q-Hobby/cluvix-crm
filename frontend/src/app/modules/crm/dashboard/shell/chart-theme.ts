import { computed, inject, Signal } from '@angular/core';

import { LayoutService } from '@/layout/service/layout.service';

/**
 * Theme ECharts theo dark mode của app. Directive `@/share/directive/echart` chỉ đọc `theme` LÚC KHỞI TẠO ⇒ template
 * bọc biểu đồ trong `@for (t of [theme()]; track t)` để dựng lại khi đổi theme (story-08 AC3).
 */
export function injectChartTheme(): Signal<'light' | 'dark'> {
  const layout = inject(LayoutService);
  return computed(() => (layout.isDarkTheme() ? 'dark' : 'light'));
}
