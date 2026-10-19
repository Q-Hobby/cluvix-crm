import { ChangeDetectionStrategy, Component, DestroyRef, inject, ViewChild } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Subject } from 'rxjs';
import { throttleTime } from 'rxjs/operators';

import { CustomerFormV2 } from '@/modules/crm/customer/form/form-customer/customer-form-v2';
import { GlobalSearch } from '@/modules/utilities/global-search/global-search';
import { BaseApp } from '@/share/base/BaseApp';
import { PERMISSION_SELECT } from '@/define';
import { ListShareImports } from '@/share/import/list-share-imports';

// Cụm 2 nút "Tìm kiếm khách hàng" + "Tạo mới khách hàng" — trích từ app.topbar.ts để DÙNG CHUNG với
// omnichat-topbar.ts (user chốt 2026-08-16: omnichat là layout riêng, không có topbar ERP nhưng vẫn
// cần 2 thao tác này). Cùng lý do/pattern như user-account-menu.ts: component TỰ CHỨA style
// (`.tb-ic`) — KHÔNG dựa vào ambient CSS `.layout-topbar-items > li > a` của _topbar_main.scss vì
// selector đó dùng `>` combinator, chèn thêm 1 lớp DOM (host component) là gãy.
// Màu icon/hover đọc từ token topbar ERP có FALLBACK trắng: omnichat-topbar không nằm trong
// `.layout-topbar-*` nên token không tồn tại ở đó, fallback cho nền gradient brand tối.
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-topbar-customer-actions',
  standalone: true,
  imports: [...ListShareImports, GlobalSearch],
  template: `
    <a
      *hasPerm="permission.permissionSearchCustomerGlobal"
      class="tb-ic tb-ic-search"
      (click)="globalSearchDialog.open()"
      [pTooltip]="'layout.app_topbar.search_customer_tooltip' | translate"
      tooltipPosition="top"
    >
      <i class="pi pi-search"></i>
    </a>
    <a
      *hasPerm="permission.permissionCreateCustomer"
      class="tb-ic tb-ic-add"
      (click)="openCreateCustomerModel(); $event.stopImmediatePropagation()"
      [pTooltip]="'layout.app_topbar.add_customer_tooltip' | translate"
      tooltipPosition="top"
      aria-keyshortcuts="Meta+Control+C Alt+C"
    >
      <i class="pi pi-user-plus"></i>
    </a>
    <app-utilities-global-search #globalSearchDialog></app-utilities-global-search>
  `,
  host: { class: 'inline-flex items-center gap-4' },
  styleUrl: './topbar-customer-actions.scss',
})
export class TopbarCustomerActions extends BaseApp {
  @ViewChild('globalSearchDialog') globalSearchDialog!: GlobalSearch;
  permission = PERMISSION_SELECT;

  private readonly destroyRef = inject(DestroyRef);
  private customerDialogRef?: any;
  // throttle 500ms: chặn double-click / phím tắt bắn liên tiếp mở 2 dialog.
  private openCustomerDialog$ = new Subject<void>();

  constructor() {
    super();
    this.openCustomerDialog$.pipe(throttleTime(500), takeUntilDestroyed()).subscribe(() => {
      this.openCustomerDialogInternal();
    });
  }

  /**
   * Tạo mới khách hàng
   */
  openCreateCustomerModel() {
    this.openCustomerDialog$.next();
  }

  private openCustomerDialogInternal() {
    // Prevent nếu dialog đang mở
    if (this.customerDialogRef) return;

    this.customerDialogRef = this.dialogWrapperService.open(CustomerFormV2, {
      header: this.translate.instant('common.label.add_new_customer'),
      width: '960px',
    });

    this.customerDialogRef.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((result: any) => {
      this.customerDialogRef = undefined;
      if (result) {
        // Truyền customer qua state để detail page paint instant + tránh tab con
        // (VD payment) gọi API thiếu customer_id trong lúc chờ GET model resolve.
        this.route.navigate([`/customer/detail/${result.id}`], { state: { customer: result } });
      }
    });
  }
}
