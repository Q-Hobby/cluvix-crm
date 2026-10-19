import { DestroyRef, inject, Service, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { fromEvent } from 'rxjs';
import { debounceTime } from 'rxjs/operators';

@Service()
export class TableLayoutService {
  readonly tableDetail = signal(this.calculateTableDetail());
  readonly tableMenuBar = signal(this.calculateTableMenuBar());
  private destroyRef = inject(DestroyRef);

  constructor() {
    fromEvent(window, 'resize')
      .pipe(debounceTime(150), takeUntilDestroyed(this.destroyRef))
      .subscribe(() => {
        this.tableDetail.set(this.calculateTableDetail());
        this.tableMenuBar.set(this.calculateTableMenuBar());
      });
  }

  private calculateTableDetail(): number {
    const width = window.innerWidth;
    if (width >= 3840) return 380; // 4K: tăng thêm để tận dụng không gian
    if (width >= 2560) return 350; // 2K: tăng từ 342
    if (width >= 1920) return 297; // Full HD
    if (width >= 1512) return 280;
    if (width >= 1440) return 265;
    if (width >= 1398) return 265;
    if (width >= 1366) return 260;
    if (width >= 1280) return 255;
    if (width >= 1024) return 245;
    if (width >= 768) return 230;
    if (width >= 640) return 220;
    return 265;
  }

  private calculateTableMenuBar(): number {
    const width = window.innerWidth;
    if (width >= 3840) return 177;
    if (width >= 2560) return 177;
    if (width >= 1920) return 164;
    if (width >= 1512) return 150;
    if (width >= 1440) return 152;
    if (width >= 1024) return 152;
    if (width >= 768) return 152;
    if (width >= 640) return 152;
    return 152;
  }
}
