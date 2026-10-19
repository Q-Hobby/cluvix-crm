import { Directive, DestroyRef, inject, input, effect, signal } from '@angular/core';

const DEFAULT_DURATION_MS = 700;

// Animate số từ giá trị hiện tại → giá trị đích bằng requestAnimationFrame.
// Dùng qua template reference: <span appCountUp [appCountUp]="val" #cu="appCountUp">{{ cu.displayValue() }}</span>
@Directive({
  selector: '[appCountUp]',
  exportAs: 'appCountUp',
})
export class CountUpDirective {
  appCountUp = input<number>(0);
  countUpDuration = input(DEFAULT_DURATION_MS);

  displayValue = signal(0);

  private destroyRef = inject(DestroyRef);
  private frameId: number | null = null;
  private reducedMotion = typeof window !== 'undefined'
    && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  private hasAnimatedOnce = false;

  constructor() {
    effect(() => {
      const target = this.appCountUp();
      this.animateTo(target);
    });
    this.destroyRef.onDestroy(() => this.cancel());
  }

  private animateTo(rawTarget: number): void {
    this.cancel();
    // Nguồn có thể truyền undefined/NaN khi data chưa sẵn sàng (vd stats() = {} lúc lỗi) — fallback 0
    // tránh hiển thị "NaN" trên KPI card.
    const target = Number.isFinite(rawTarget) ? rawTarget : 0;
    const start = this.hasAnimatedOnce ? this.displayValue() : 0;
    this.hasAnimatedOnce = true;

    if (this.reducedMotion || start === target) {
      this.displayValue.set(target);
      return;
    }

    const duration = this.countUpDuration();
    const startTime = performance.now();
    const step = (now: number) => {
      const progress = Math.min((now - startTime) / duration, 1);
      // Làm tròn khi đang animate (đếm số nguyên mượt mắt), nhưng set ĐÚNG target
      // (không làm tròn) ở frame cuối — tránh mất phần thập phân tiền tệ.
      this.displayValue.set(progress < 1 ? Math.round(start + (target - start) * progress) : target);
      this.frameId = progress < 1 ? requestAnimationFrame(step) : null;
    };
    this.frameId = requestAnimationFrame(step);
  }

  private cancel(): void {
    if (this.frameId !== null) {
      cancelAnimationFrame(this.frameId);
      this.frameId = null;
    }
  }
}
