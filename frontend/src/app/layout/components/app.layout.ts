import { DOCUMENT, NgClass } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  computed,
  DestroyRef,
  effect,
  inject,
  Renderer2,
  ViewChild,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { NavigationEnd, Router, RouterOutlet } from '@angular/router';
import { filter } from 'rxjs';

import { AppBreadcrumb } from '@/layout/components/app.breadcrumb';
import { AppSidebar } from '@/layout/components/app.sidebar';
import { AppTopbar } from '@/layout/components/app.topbar';
import { LayoutService } from '@/layout/service/layout.service';
import { TimezoneDialog } from '@/modules/utilities/timezone-dialog/timezone-dialog';

/** Class trên `<body>` khoá cuộn trang khi sidebar mobile đang mở. */
const BLOCKED_SCROLL_CLASS = 'blocked-scroll';

/**
 * Khung trang sau đăng nhập: topbar + sidebar + vùng nội dung (breadcrumb, router-outlet).
 * Class trên `.layout-container` lấy từ `LayoutService`; khi sidebar nổi (overlay/mobile) đang mở,
 * click ra ngoài hoặc điều hướng sang trang khác sẽ đóng nó.
 */
@Component({
  selector: 'app-layout',
  standalone: true,
  imports: [NgClass, RouterOutlet, AppTopbar, AppSidebar, AppBreadcrumb, TimezoneDialog],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="layout-container" [ngClass]="containerClass()">
      <div app-topbar></div>
      <div app-sidebar></div>
      <div class="layout-content-wrapper">
        <div app-breadcrumb></div>
        <div class="layout-content">
          <router-outlet />
          <app-timezone-dialog />
        </div>
      </div>
    </div>
  `,
})
export class AppLayout {
  private readonly layoutService = inject(LayoutService);
  private readonly renderer = inject(Renderer2);
  private readonly router = inject(Router);
  private readonly document = inject(DOCUMENT);
  private readonly destroyRef = inject(DestroyRef);

  @ViewChild(AppSidebar) appSidebar?: AppSidebar;
  @ViewChild(AppTopbar) appTopbar?: AppTopbar;

  /** Gỡ listener click trên document; `null` = chưa gắn. */
  private unlistenDocumentClick: (() => void) | null = null;

  readonly containerClass = computed(() => {
    const mode = this.layoutService.effectiveMenuMode();
    const state = this.layoutService.layoutState();
    const overlay = mode === 'overlay';
    return {
      'layout-static': !overlay,
      'layout-overlay': overlay,
      'layout-static-inactive': !overlay && state.staticMenuDesktopInactive,
      'layout-overlay-active': state.overlayMenuActive,
      'layout-mobile-active': state.staticMenuMobileActive,
      'layout-topbar-cluvix': true,
    };
  });

  /** Sidebar nổi (overlay desktop hoặc mobile) đang mở. */
  private readonly floatingMenuOpen = computed(() => {
    const state = this.layoutService.layoutState();
    return state.overlayMenuActive || state.staticMenuMobileActive;
  });

  constructor() {
    this.layoutService.overlayOpen$.pipe(takeUntilDestroyed(this.destroyRef)).subscribe(() => {
      if (!this.unlistenDocumentClick) {
        this.unlistenDocumentClick = this.renderer.listen(
          this.document,
          'click',
          (event: MouseEvent) => {
            if (this.isOutsideClick(event)) this.hideMenu();
          },
        );
      }
      if (this.layoutService.layoutState().staticMenuMobileActive) {
        this.renderer.addClass(this.document.body, BLOCKED_SCROLL_CLASS);
      }
    });

    this.router.events
      .pipe(
        filter((e) => e instanceof NavigationEnd),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe(() => this.hideMenu());

    // Đóng bằng chính nút menu (không qua hideMenu) cũng phải gỡ listener + mở khoá cuộn.
    effect(() => {
      if (this.floatingMenuOpen()) return;
      this.unbindDocumentClick();
      this.renderer.removeClass(this.document.body, BLOCKED_SCROLL_CLASS);
    });

    this.destroyRef.onDestroy(() => {
      this.unbindDocumentClick();
      this.renderer.removeClass(this.document.body, BLOCKED_SCROLL_CLASS);
    });
  }

  /** Click không nằm trong menu sidebar và không phải nút mở menu trên topbar. */
  private isOutsideClick(event: MouseEvent): boolean {
    const target = event.target as Node | null;
    if (!target) return false;
    const menuEl: HTMLElement | undefined = this.appSidebar?.appMenu?.el?.nativeElement;
    const buttonEl: HTMLElement | undefined = this.appTopbar?.menuButton?.nativeElement;
    const inMenu = !!menuEl && (menuEl === target || menuEl.contains(target));
    const onButton = !!buttonEl && (buttonEl === target || buttonEl.contains(target));
    return !inMenu && !onButton;
  }

  private hideMenu(): void {
    this.layoutService.layoutState.update((s) => ({
      ...s,
      overlayMenuActive: false,
      staticMenuMobileActive: false,
    }));
    this.layoutService.reset();
    this.unbindDocumentClick();
    this.renderer.removeClass(this.document.body, BLOCKED_SCROLL_CLASS);
  }

  private unbindDocumentClick(): void {
    this.unlistenDocumentClick?.();
    this.unlistenDocumentClick = null;
  }
}
