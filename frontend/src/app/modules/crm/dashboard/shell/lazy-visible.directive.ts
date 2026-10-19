import { DestroyRef, Directive, ElementRef, afterNextRender, inject, output } from '@angular/core';

import { observeOnce } from './observe-once.util';

/**
 * Phát `appLazyVisible` ĐÚNG 1 lần khi phần tử vào gần viewport (600px). Ngừng theo dõi khi component bị
 * huỷ — thay `dashboard.ts::setupLazyLoading` cũ (không disconnect nếu user không cuộn tới).
 */
@Directive({ selector: '[appLazyVisible]' })
export class LazyVisibleDirective {
  readonly appLazyVisible = output<void>();
  private readonly el = inject<ElementRef<HTMLElement>>(ElementRef);

  constructor() {
    const destroyRef = inject(DestroyRef);
    afterNextRender(() => {
      const stop = observeOnce(this.el.nativeElement, () => this.appLazyVisible.emit());
      destroyRef.onDestroy(stop);
    });
  }
}
