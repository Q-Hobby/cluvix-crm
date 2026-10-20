import { isPlatformBrowser } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  ElementRef,
  inject,
  PLATFORM_ID,
  ViewChild,
} from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { TranslateService } from '@ngx-translate/core';
import { Popover } from 'primeng/popover';
import { NavigationEnd } from '@angular/router';
import { filter, map } from 'rxjs/operators';

import { ReminderMineTodayList } from '@/modules/crm/reminder/today-list/reminder-mine-today-list';
import { PwaInstallService } from '@/core/services/pwa-install.service';
import { ReminderAlertService } from '@/core/services/reminder-alert.service';
import { LayoutService } from '@/layout/service/layout.service';
import { BaseApp } from '@/share/base/BaseApp';
import { buildDocsUrl, isDocsConfigured, PERMISSION_SELECT } from '@/define';
import { ListShareImports } from '@/share/import/list-share-imports';
import { UserAccountMenu } from '@/layout/components/user-account-menu';
import { TopbarCustomerActions } from '@/layout/components/topbar-customer-actions';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: '[app-topbar]',
  standalone: true,
  imports: [...ListShareImports, UserAccountMenu, TopbarCustomerActions],
  template: `
    <div class="layout-topbar-start">
      <a routerLink="/" class="layout-topbar-logo">
        @defer (on idle) {
          <img
            src="/assets/logo/logo-ngang.svg"
            alt="Company logo"
            class="logo-custom"
            decoding="async"
            fetchpriority="low"
          />
        }
      </a>
      <a #menuButton class="layout-menu-button" (click)="onMenuButtonClick()">
        <i class="pi pi-chevron-right"></i>
      </a>
    </div>

    <div class="layout-topbar-end">
      <div class="layout-topbar-actions-start flex items-center gap-2 ml-3">
        <!-- Link docs theo ngữ cảnh: mở bài ứng với màn hình đang đứng (map ở docs.define.ts).
             Tab mới để không mất dữ liệu form đang nhập dở. CHỈ desktop (chủ sản phẩm chốt
             2026-09-22): isDesktop() theo thiết bị + CSS ẩn ≤991px khi desktop thu nhỏ cửa sổ. -->
        @if (isDesktop()) {
          @if (docsEnabled) {
          <a
            [href]="docsUrl()"
            target="_blank"
            rel="noopener noreferrer"
            class="topbar-docs-link"
            [pTooltip]="'layout.app_topbar.docs_tooltip' | translate"
            tooltipPosition="bottom"
            [attr.aria-label]="'layout.app_topbar.docs_label' | translate"
          >
            <i class="pi pi-book"></i>
            <span>{{ 'layout.app_topbar.docs_label' | translate }}</span>
            <i class="pi pi-external-link topbar-docs-link-ext"></i>
          </a>
          }
          <!-- Cài PWA lên máy (xem pwa-install.service.ts). Tự biến mất khi đã cài / đang chạy
               trong app đã cài / trình duyệt không hỗ trợ. Không gọi được hộp thoại cài (Safari, hoặc
               Chrome sau khi user đã Hủy 1 lần) ⇒ popover hướng dẫn cài tay theo trình duyệt. -->
          @if (pwaInstall.showInstall()) {
            <button
              type="button"
              class="topbar-install-btn"
              [pTooltip]="'layout.app_topbar.install_tooltip' | translate"
              tooltipPosition="bottom"
              (click)="onInstallClick($event, installGuide)"
            >
              <i class="pi pi-download"></i>
              <span>{{ 'layout.app_topbar.install_label' | translate }}</span>
            </button>
            <p-popover #installGuide>
              <div class="w-72 flex flex-col gap-3">
                @if (pwaInstall.isSafariMac) {
                  <div class="font-semibold">{{ 'layout.app_topbar.install_safari_title' | translate }}</div>
                  <ol class="list-decimal pl-5 flex flex-col gap-2 text-sm">
                    <li>{{ 'layout.app_topbar.install_safari_step1' | translate }}</li>
                    <li>{{ 'layout.app_topbar.install_safari_step2' | translate }}</li>
                  </ol>
                } @else {
                  <div class="font-semibold">{{ 'layout.app_topbar.install_chrome_title' | translate }}</div>
                  <ol class="list-decimal pl-5 flex flex-col gap-2 text-sm">
                    <li>{{ 'layout.app_topbar.install_chrome_step1' | translate }}</li>
                    <li>{{ 'layout.app_topbar.install_chrome_step2' | translate }}</li>
                  </ol>
                }
              </div>
            </p-popover>
          }
        }
      </div>
      <div class="layout-topbar-actions-end">
        <ul class="layout-topbar-items">
          <li>
            <app-topbar-customer-actions />
          </li>
          <li class="tb-user-li">
            <app-user-account-menu [showName]="true" />
          </li>
        </ul>
      </div>
    </div>
  `,
  host: {
    class: 'layout-topbar',
  },
  styleUrl: './app.topbar.scss',
})
export class AppTopbar extends BaseApp {
  layoutService = inject(LayoutService);
  reminderAlertService = inject(ReminderAlertService);
  protected readonly pwaInstall = inject(PwaInstallService);
  el = inject(ElementRef);
  @ViewChild('menuButton') menuButton!: ElementRef<HTMLButtonElement>;
  private readonly _ts = inject(TranslateService);
  permission = PERMISSION_SELECT;
  // URL bài docs cho màn hình hiện tại — tính lại khi đổi route hoặc đổi ngôn ngữ.
  private readonly _currentUrl = toSignal(
    this.route.events.pipe(
      filter((e): e is NavigationEnd => e instanceof NavigationEnd),
      map(e => e.urlAfterRedirects),
    ),
    { initialValue: this.route.url },
  );
  private readonly _currentLang = toSignal(
    this._ts.onLangChange.pipe(map(e => e.lang)),
    { initialValue: this._ts.currentLang },
  );
  protected readonly docsEnabled = isDocsConfigured();
  protected readonly docsUrl = computed(() => buildDocsUrl(this._currentUrl(), this._currentLang()));
  private readonly isBrowser = isPlatformBrowser(inject(PLATFORM_ID));
  private reminderDialogOpened = false;

  constructor() {
    super();

    effect(() => {
      const count = this.reminderAlertService.getCount();
      if (this.isBrowser && count > 0 && !this.reminderDialogOpened) {
        this.reminderDialogOpened = true;
        queueMicrotask(() => {
          this.openMyToDayReminder();
        });
      }
    });
  }

  onMenuButtonClick() {
    this.layoutService.onMenuToggle();
  }

  /**
   * Chrome/Edge: mở hộp thoại cài của trình duyệt. Không gọi được (Safari, hoặc Chrome sau khi user
   * đã Hủy 1 lần trong lần tải trang này) ⇒ mở popover hướng dẫn cài tay.
   */
  async onInstallClick(event: Event, guide: Popover) {
    // Giữ target TRƯỚC await — sau await event.currentTarget = null, popover mất chỗ neo.
    const anchor = event.currentTarget as HTMLElement;
    const result = await this.pwaInstall.install();
    if (result === 'unavailable') {
      guide.toggle(event, anchor);
    }
  }

  /**
   * Mở hộp thoại lời nhắc
   */
  openMyToDayReminder() {
    this.dialogWrapperService.open(ReminderMineTodayList, {
      header: this.translate.instant('layout.app_topbar.today_reminder_dialog_header'),

      width: '960px',
      contentStyle: { overflow: 'auto', padding: '0' },
    });
  }

}
