import { inject, Service, signal, Signal, WritableSignal } from '@angular/core';
import {
  NavigationCancel,
  NavigationEnd,
  NavigationError,
  NavigationStart,
  Router,
} from '@angular/router';
import { filter } from 'rxjs/operators';

// Thanh progress top-bar cho GET + navigation — tách khỏi GlobalLoadingService (chỉ track CUD)
// vì 2 khái niệm UX khác nhau: nav/GET cần feedback tức thời gắn liền route,
// không phải overlay chặn tương tác như spinner CUD.
//
// Gốc rễ của "bấm menu mạng chậm không phản ứng gì": route lazy-load qua import()
// (SelectivePreloadStrategy không preload hết) — chunk tải chậm xảy ra TRƯỚC khi
// component đích kịp gọi GET nào, nên chỉ track HttpClient là không đủ. Phải nghe
// thẳng Router NavigationStart (bắn ngay lúc click, trước cả lúc chunk bắt đầu tải)
// tới NavigationEnd/Cancel/Error để phủ toàn bộ pipeline: tải chunk + resolver + init.
@Service()
export class RouteProgressService {
  private pending = 0;

  // Ngưỡng giống GlobalLoadingService — tránh chớp nháy khi mạng nhanh.
  private readonly SHOW_DELAY_MS = 150;
  private readonly MIN_VISIBLE_MS = 250;

  private showTimer: ReturnType<typeof setTimeout> | null = null;
  private hideTimer: ReturnType<typeof setTimeout> | null = null;
  private shownAt = 0;

  private readonly _visible: WritableSignal<boolean> = signal(false);
  public readonly visible: Signal<boolean> = this._visible.asReadonly();

  constructor() {
    const router = inject(Router);
    // Service singleton sống suốt vòng đời app → không cần takeUntilDestroyed.
    router.events
      .pipe(
        filter(
          (e) =>
            e instanceof NavigationStart ||
            e instanceof NavigationEnd ||
            e instanceof NavigationCancel ||
            e instanceof NavigationError,
        ),
      )
      .subscribe((e) => (e instanceof NavigationStart ? this.start() : this.stop()));
  }

  start(): void {
    this.pending++;
    if (this.pending === 1) this.scheduleVisibility(true);
  }

  stop(): void {
    this.pending = Math.max(0, this.pending - 1);
    if (this.pending === 0) this.scheduleVisibility(false);
  }

  private scheduleVisibility(shouldShow: boolean): void {
    if (shouldShow) {
      if (this._visible() || this.showTimer) return;
      if (this.hideTimer) {
        clearTimeout(this.hideTimer);
        this.hideTimer = null;
      }
      this.showTimer = setTimeout(() => {
        this.showTimer = null;
        this._visible.set(true);
        this.shownAt = Date.now();
      }, this.SHOW_DELAY_MS);
      return;
    }

    if (this.showTimer) {
      clearTimeout(this.showTimer);
      this.showTimer = null;
      return; // Chưa kịp hiện — không cần làm gì thêm.
    }
    if (!this._visible() || this.hideTimer) return;

    const remaining = Math.max(0, this.MIN_VISIBLE_MS - (Date.now() - this.shownAt));
    this.hideTimer = setTimeout(() => {
      this.hideTimer = null;
      this._visible.set(false);
    }, remaining);
  }
}
