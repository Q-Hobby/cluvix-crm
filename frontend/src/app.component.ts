// app.component.ts
import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  inject,
  NgZone,
  OnInit,
} from '@angular/core';
import { RouterModule } from '@angular/router';
import { TranslateService } from '@ngx-translate/core';
import { PrimeNG } from 'primeng/config';
import { ConfirmPopupModule } from 'primeng/confirmpopup';
import { Toast } from 'primeng/toast';
import { animationFrameScheduler, fromEvent } from 'rxjs';
import { filter, map, take, takeUntil, throttleTime } from 'rxjs/operators';

import { AppLoader } from './app-loader';
import { RouteProgressBar } from './route-progress-bar';
import { isLoggedIn, userProfile } from '@/core/services/auth';
import { TokenService } from '@/core/services/auth/token.service';

import { GlobalSearchService } from '@/modules/utilities/global-search/global-search.service';
import { CreateCustomerDialogService } from '@/modules/utilities/create-customer-dialog.service';
import { CommandMenuService } from '@/modules/utilities/command-menu/command-menu.service';
import { CommandMenu } from '@/modules/utilities/command-menu/command-menu';
import { hasPermission, hasAnyPermission } from '@/core/services/auth/auth.selectors';
import { PERMISSION_SELECT } from '@/define';
import { NotificationService } from '@/share/services/notification.service';
import { PwaInstallService } from '@/core/services/pwa-install.service';

@Component({
  selector: 'cluvix',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [RouterModule, Toast, AppLoader, RouteProgressBar, ConfirmPopupModule, CommandMenu],
  template: `
    <div translate="no">
      <app-loader></app-loader>
      <app-route-progress-bar></app-route-progress-bar>
      <p-toast position="top-right" [baseZIndex]="10000" />
      <p-confirmpopup></p-confirmpopup>
      @if (isLoggedIn()) {
        <app-utilities-command-menu></app-utilities-command-menu>
      }
      <router-outlet />
    </div>
  `,
})
export class AppComponent implements OnInit {
  private translateService = inject(TranslateService);
  private primeng = inject(PrimeNG);
  private ngZone = inject(NgZone);
  private destroyRef = inject(DestroyRef);

  private globalSearchService = inject(GlobalSearchService);
  private createCustomerDialog = inject(CreateCustomerDialogService);
  private commandMenuService = inject(CommandMenuService);
  private tokenService = inject(TokenService);
  private noti = inject(NotificationService);
  // Inject sớm để bắt `beforeinstallprompt` (bắn trước khi topbar render) — xem pwa-install.service.ts.
  protected readonly pwaInstall = inject(PwaInstallService);

  protected readonly isLoggedIn = isLoggedIn;

  ngOnInit(): void {
    // Default lang init đã chạy trong APP_INITIALIZER (app.config.ts initializeTranslations)
    // → translations đã loaded TRƯỚC khi component render → no FOUC.
    // Sau đăng nhập: nếu user.locale khác lang hiện tại thì switch (chỉ khi user đã đăng nhập).
    const hasValidAuth = isLoggedIn() && this.tokenService.hasValidToken;

    if (hasValidAuth) {
      const locale = userProfile()?.locale;
      if (locale && this.translateService.currentLang !== locale) {
        this.translateService.use(locale);
      }
      this.translateService
        .get('primeng')
        .pipe(takeUntil(this.onDestroy()))
        .subscribe({
          next: (res: any) => {
            if (res && typeof res === 'object') {
              this.primeng.setTranslation(res);
            }
          },
          error: (err) => console.warn('[AppComponent] PrimeNG translation failed:', err),
        });
    }

    this.setupShortcuts();
    this.showPostReloadToast();
  }

  private showPostReloadToast(): void {
    const detail = sessionStorage.getItem('post_reload_toast');
    if (!detail) return;
    sessionStorage.removeItem('post_reload_toast');
    // Defer 1 tick: p-toast cần hoàn thành ngAfterViewInit + subscribe MessageService
    // trước khi nhận message. Gọi sớm hơn (ngOnInit) → Subject emit trước khi
    // p-toast subscribe → message bị drop silently.
    setTimeout(() => {
      this.ngZone.run(() => {
        this.translateService.get(['common.label.notification', detail])
          .pipe(take(1))
          .subscribe(t => this.noti.success(t['common.label.notification'], t[detail]));
      });
    }, 0);
  }

  /** Rx helper to complete on destroy */
  private onDestroy() {
    const controller = new AbortController();
    this.destroyRef.onDestroy(() => controller.abort());
    // Adapter nhỏ để dùng với takeUntil:
    const { signal } = controller;
    return fromEvent(signal as any, 'abort'); // emit 1 lần khi destroy
  }

  private setupShortcuts() {
    this.ngZone.runOutsideAngular(() => {
      fromEvent<KeyboardEvent>(window, 'keydown', { capture: true })
        .pipe(
          throttleTime(0, animationFrameScheduler, { leading: true, trailing: false }),
          filter((ev) => !ev.repeat && !ev.defaultPrevented),
          map((ev) => {
            const target = ev.target as HTMLElement | null;
            const tag = target?.tagName ?? '';
            if (['INPUT', 'TEXTAREA', 'SELECT'].includes(tag) || target?.isContentEditable)
              return null;

            const isMac = /Mac|iPhone|iPod|iPad/.test(navigator.userAgent);
            const code = ev.code;

            const isSearchKey =
              (isMac && ev.metaKey && code === 'KeyF') || (!isMac && ev.altKey && code === 'KeyF');

            const isCreateCustomer =
              (isMac && ev.metaKey && ev.ctrlKey && code === 'KeyC') ||
              (!isMac && ev.altKey && code === 'KeyC');

            const isCommandMenu =
              (isMac && ev.metaKey && code === 'KeyK') || (!isMac && ev.ctrlKey && code === 'KeyK');

            return { ev, isSearchKey, isCreateCustomer, isCommandMenu };
          }),
          filter(Boolean),
          takeUntil(this.onDestroy()),
        )
        .subscribe(({ ev, isSearchKey, isCreateCustomer, isCommandMenu }: any) => {
          if (isSearchKey) {
            ev.preventDefault();
            ev.stopPropagation();
            ev.stopImmediatePropagation();
            if (hasPermission(PERMISSION_SELECT.permissionSearchCustomerGlobal)) {
              requestAnimationFrame(() => {
                this.ngZone.run(() => this.globalSearchService.toggle());
              });
            }
          } else if (isCreateCustomer) {
            ev.preventDefault();
            ev.stopPropagation();
            ev.stopImmediatePropagation();
            if (hasAnyPermission(
              PERMISSION_SELECT.permissionCreateCustomer,
            )) {
              requestAnimationFrame(() => {
                this.ngZone.run(() => this.createCustomerDialog.toggle());
              });
            }
          } else if (isCommandMenu) {
            ev.preventDefault();
            ev.stopPropagation();
            ev.stopImmediatePropagation();
            if (isLoggedIn() && this.tokenService.hasValidToken) {
              requestAnimationFrame(() => {
                this.ngZone.run(() => this.commandMenuService.toggle());
              });
            }
          }
        });
    });
  }
}
