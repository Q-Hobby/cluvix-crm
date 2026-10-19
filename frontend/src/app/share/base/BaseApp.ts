import { Directive, Injector, inject, signal } from '@angular/core';
import { FormBuilder } from '@angular/forms';
import { Router } from '@angular/router';
import { TranslateService } from '@ngx-translate/core';
import { ConfirmationService } from 'primeng/api';
import { DynamicDialogRef } from 'primeng/dynamicdialog';
import { Observable } from 'rxjs';

import { DialogWrapperService } from '@/share/services/dialog-wrapper.service';
import { GlobalLoadingService } from '@/share/services/global-loading.service';
import { NotificationService } from '@/share/services/notification.service';
import { PermissionsService } from '@/share/services/permissions.service';
import { StorageService } from '@/share/services/storage.service';
import { TableLayoutService } from '@/share/services/table-layout.service';
import {AuthService, RbacService} from "@/core/services/auth";
import { TimezoneService } from '@/core/services/timezone.service';
import { ExportFileService } from '@/core/services/export-file.service';

@Directive()
export class BaseApp {
  isSubmitting = signal<boolean>(false);
  loading = signal(true);
  isDesktop = signal(false);
  isTablet = signal(false);
  isMobile = signal(false);
  protected authService: AuthService = inject(AuthService);
  protected rbacService: RbacService = inject(RbacService);
  protected timezoneService: TimezoneService = inject(TimezoneService);
  protected translate: TranslateService = inject(TranslateService);
  protected permissionsService: PermissionsService = inject(PermissionsService);
  protected noti: NotificationService = inject(NotificationService);
  protected confirmationService: ConfirmationService = inject(ConfirmationService);
  protected route: Router = inject(Router);
  protected dialogWrapperService: DialogWrapperService = inject(DialogWrapperService);
  protected fb: FormBuilder = inject(FormBuilder);
  protected dialogRef: DynamicDialogRef = inject(DynamicDialogRef);
  protected storageService: StorageService = inject(StorageService);
  protected loadingService = inject(GlobalLoadingService);
  private tableLayoutService = inject(TableLayoutService);
  // Lấy ExportFileService LƯỜI qua Injector (chỉ khi thật sự export) — tránh buộc mọi spec của
  // component kế thừa BaseApp phải provide HttpClient.
  private readonly baseInjector = inject(Injector);
  readonly tableMenuBar = this.tableLayoutService.tableMenuBar;
  readonly tableDetail = this.tableLayoutService.tableDetail;
  readonly today = this.timezoneService.now();

  constructor() {
    this.detectDeviceType();
    // console.log(window.innerWidth);
  }

  protected detectDeviceType() {
    const userAgent =
      navigator.userAgent || navigator.vendor || ('opera' in window ? (window as any).opera : '');
    const isTouchDevice = navigator.maxTouchPoints > 0;

    if (/iPad/.test(userAgent) || (isTouchDevice && /Macintosh/.test(userAgent))) {
      this.isTablet.set(true);
    } else if (/Android/.test(userAgent) && !/Mobile/.test(userAgent)) {
      this.isTablet.set(true);
    } else if (/iPhone|Android/.test(userAgent)) {
      this.isMobile.set(true);
    } else {
      this.isDesktop.set(true);
    }
  }

  // Mở iframe in ẩn — dùng chung cho mọi component cần in
  protected printCommon(url: string): boolean {
    const iframe = document.createElement('iframe');
    iframe.src = url;
    iframe.name = 'prints-frame-name';
    iframe.style.display = 'none';
    document.body.appendChild(iframe);
    iframe.focus();
    return false;
  }

  /**
   * Fetch data trước → lưu sessionStorage → show loading → mở iframe.
   * Loading hide khi PrintLayout gửi postMessage('print-dialog-opening').
   * @param url      - đường dẫn iframe (cacheKey sẽ tự append)
   * @param cacheKey - key lưu sessionStorage
   * @param fetch$   - Observable gọi API
   * @param validate - (optional) return false → warn + không mở print
   * @param noDataMsg
   */
  protected openPrint(
    url: string,
    cacheKey: string,
    fetch$: Observable<any>,
    validate?: (data: any) => boolean,
    noDataMsg = 'Không có dữ liệu để in',
  ): void {
    this.loadingService.showLoading('openPrint');
    fetch$.subscribe({
      next: (data: any) => {
        if (validate && !validate(data)) {
          this.loadingService.hideLoading('openPrint');
          this.noti.warnMsg(noDataMsg);
          return;
        }
        sessionStorage.setItem(cacheKey, JSON.stringify(data));
        const onMsg = (e: MessageEvent) => {
          if (e.data === 'print-dialog-opening') {
            window.removeEventListener('message', onMsg);
            this.loadingService.hideLoading('openPrint');
          }
        };
        window.addEventListener('message', onMsg);
        const sep = url.includes('?') ? '&' : '?';
        this.printCommon(`${url}${sep}cacheKey=${cacheKey}`);
      },
      error: () => this.loadingService.hideLoading('openPrint'),
    });
  }

  /**
   * Giống openPrint nhưng dùng window.open (tab mới) thay vì iframe ẩn.
   * Loading hide ngay sau khi tab mới mở (không cần chờ print dialog).
   */
  protected openPrintWindow(
    url: string,
    cacheKey: string,
    fetch$: Observable<any>,
    windowOptions = 'width=1200,height=800',
  ): void {
    this.loadingService.showLoading('openPrint');
    fetch$.subscribe({
      next: (data: any) => {
        sessionStorage.setItem(cacheKey, JSON.stringify(data));
        const sep = url.includes('?') ? '&' : '?';
        window.open(`${url}${sep}cacheKey=${cacheKey}`, '_blank', windowOptions);
        this.loadingService.hideLoading('openPrint');
      },
      error: () => this.loadingService.hideLoading('openPrint'),
    });
  }

  /**
   * Tải file export BE trả về (`?_export=1` → URL `/api/download/export/<tên>`). Endpoint CÓ JWT nên
   * KHÔNG được `window.location.href = url` (thiếu header Authorization ⇒ 401) — xem ExportFileService.
   */
  protected downloadExport(url: string | null | undefined): void {
    this.baseInjector.get(ExportFileService).download(url);
  }
}
