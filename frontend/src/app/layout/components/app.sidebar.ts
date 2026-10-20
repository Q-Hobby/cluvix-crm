import { ChangeDetectionStrategy, Component, computed, ElementRef, inject, OnDestroy, ViewChild } from '@angular/core';

import { AppMenu } from './app.menu';

import { LayoutService } from '@/layout/service/layout.service';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: '[app-sidebar]',
  standalone: true,
  imports: [AppMenu],
  template: ` <div class="layout-sidebar">
    <div class="layout-sidebar-top">
      <a href="/">
        <img
          [src]="'/assets/logo/icon.png'"
          alt="Company logo"
          class="h-[40px] w-auto"
          decoding="async"
          fetchpriority="low"
        />
      </a>
    </div>
    <!-- Khối tài khoản (app-menu-profile) đã gỡ khỏi sidebar (user chốt 2026-10-19): menu tài khoản
         trên topbar (user-account-menu) là lối vào DUY NHẤT và có đủ mọi mục. -->
    <div #menuContainer class="layout-menu-container">
      <div app-menu></div>
    </div>
  </div>`,
})
export class AppSidebar implements OnDestroy {
  el = inject(ElementRef);

  layoutService = inject(LayoutService);

  @ViewChild(AppMenu) appMenu!: AppMenu;

  @ViewChild('menuContainer') menuContainer!: ElementRef;

  overlayMenuActive = computed(() => this.layoutService.layoutState().overlayMenuActive);

  resetOverlay() {
    if (this.overlayMenuActive()) {
      this.layoutService.layoutState.update((val: any) => ({ ...val, overlayMenuActive: false }));
    }
  }

  ngOnDestroy() {
    this.resetOverlay();
  }
}
