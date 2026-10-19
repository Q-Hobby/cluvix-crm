import { inject, Service, signal, Signal, WritableSignal } from '@angular/core';
import { NavigationEnd, NavigationStart, Router } from '@angular/router';
import { filter } from 'rxjs/operators';

/**
 * Cho phép màn detail thay nhãn breadcrumb CUỐI bằng tên bản ghi đang xem
 * (vd "Nguyễn Văn A" thay cho "Chi tiết khách hàng").
 *
 * Nhãn tự xoá khi đổi PATH (bỏ query/fragment) — cùng lý do với setup-route-meta.ts:
 * màn detail điều hướng đổi query param để nhớ tab đang mở (`?tab=`), nếu xoá theo
 * mọi NavigationEnd thì breadcrumb sẽ nhảy về nhãn tĩnh ngay khi user chuyển tab.
 */
@Service()
export class BreadcrumbLabelService {
  private readonly _label: WritableSignal<string | null> = signal(null);
  readonly label: Signal<string | null> = this._label.asReadonly();

  private lastPath: string | null = null;

  constructor() {
    const router = inject(Router);
    this.lastPath = this.pathOf(router.url);
    // Service singleton sống suốt vòng đời app → không cần takeUntilDestroyed.
    router.events
      .pipe(filter((e) => e instanceof NavigationStart || e instanceof NavigationEnd))
      .subscribe((e) => {
        // Xoá ở NavigationStart, KHÔNG ở NavigationEnd: component đích được tạo và
        // chạy ngOnInit TRƯỚC NavigationEnd, nên màn detail paint tên từ preload
        // (history.state) sẽ bị xoá ngay sau đó nếu dọn ở NavigationEnd.
        if (e instanceof NavigationStart) {
          const path = this.pathOf(e.url);
          if (path === this.lastPath) return;
          this.lastPath = path;
          this._label.set(null);
          return;
        }
        // Chốt lại path thật sau redirect — chỉ cập nhật mốc, không xoá nhãn.
        this.lastPath = this.pathOf((e as NavigationEnd).urlAfterRedirects || (e as NavigationEnd).url);
      });
  }

  set(label: string | null | undefined): void {
    this._label.set(label?.trim() || null);
  }

  private pathOf(url: string): string {
    return url.split(/[?#]/)[0];
  }
}
