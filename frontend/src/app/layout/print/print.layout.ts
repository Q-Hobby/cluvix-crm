import { ChangeDetectionStrategy, Component, computed, effect, inject, signal } from '@angular/core';
import { RouterOutlet } from '@angular/router';

import { PrintDataService } from '@/layout/print/print-data.service';
import { PrintFooterLayout } from '@/layout/print/print-footer.layout';
import { PrintHeaderLayout } from '@/layout/print/print-header.layout';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-print-layout',
  template: `
    <div class="sheet" id="printable">
      <!-- Header Section -->
      @if (ready()) {
        <app-layout-print-header
          [branchId]="branchId()"
          (logoLoaded)="onLogoLoaded()"
        />
      } @else {
        <!-- Header Skeleton -->
        <div class="sheet_head border-b pb-1 animate-pulse" style="page-break-inside: avoid">
          <div class="flex items-start gap-4">
            <div class="flex-none w-[72px] h-[72px] bg-gray-200 rounded"></div>
            <div class="flex-1 space-y-2">
              <div class="h-5 bg-gray-200 rounded w-3/4"></div>
              <div class="h-4 bg-gray-200 rounded w-full"></div>
              <div class="h-4 bg-gray-200 rounded w-2/3"></div>
              <div class="h-4 bg-gray-200 rounded w-2/3"></div>
            </div>
          </div>
        </div>
      }

      <!-- Main Content -->
      <router-outlet />

      <!-- Footer Section -->
      @if (shouldShowFooter()) {
        <app-layout-print-footer
          [branchId]="branchId()"
          [customerName]="customerName() || ''"
          [layoutType]="layoutType()"
        />
      } @else if (ready()) {
        <!-- Footer Skeleton when data is loading -->
        <div class="sheet_foot border-t pt-2 mt-4 animate-pulse" style="page-break-inside: avoid">
          <div class="flex justify-between text-center">
            <div class="w-1/2">
              <div class="h-4 bg-gray-200 rounded w-28 mx-auto mb-1"></div>
              <!-- Kế toán trưởng -->
              <div class="h-3 bg-gray-200 rounded w-16 mx-auto mb-20"></div>
              <!-- (Chữ ký) -->
              <div class="h-3 bg-gray-200 rounded w-20 mx-auto"></div>
              <!-- Empty space -->
            </div>

            <div class="w-1/2">
              <div class="h-4 bg-gray-200 rounded w-28 mx-auto mb-1"></div>
              <!-- Người lập biểu -->
              <div class="h-3 bg-gray-200 rounded w-16 mx-auto mb-20"></div>
              <!-- (Chữ ký) -->
              <div class="h-3 bg-gray-200 rounded w-20 mx-auto"></div>
              <!-- Tên người -->
            </div>
          </div>
        </div>
      }
    </div>
  `,
  standalone: true,
  imports: [RouterOutlet, PrintHeaderLayout, PrintFooterLayout],
})
export class PrintLayout {
  readonly branchId = computed(() => this.context()?.branch ?? null);
  // Cổng hiển thị header/footer + đếm giờ in = trang in ĐÃ `setModel` (dữ liệu sẵn sàng), KHÔNG phải
  // "có chi nhánh": trang tổng hợp nhiều chi nhánh (`print-appointment-byhour-list`,
  // `print-payment-in-day`) không có 1 `branch` ⇒ trước đây (sau khi bỏ cổng `company`, story-43)
  // header kẹt skeleton và không bao giờ tự in. Thiếu chi nhánh ⇒ header dùng địa chỉ/hotline của
  // hồ sơ doanh nghiệp (D22).
  readonly ready = computed(() => this.context() != null);
  readonly customer = computed(() => this.context()?.customer ?? null);
  readonly customerName = computed(() => this.customer()?.fullname ?? null);
  readonly layoutType = computed(() => this.context()?.layoutType ?? 'default');
  // Computed for footer logic
  readonly shouldShowFooter = computed(() => {
    const hasContext = this.ready();
    const hasCustomer = !!this.customerName();
    const hasSpecialLayout = this.layoutType() !== 'default';

    return hasContext && (hasCustomer || hasSpecialLayout);
  });
  private printDataService = inject(PrintDataService);
  // Computed properties for reactive data
  readonly context = computed(() => this.printDataService.model());
  private logoIsLoaded = signal(false);
  private printTriggered = signal(false);
  private printTimeout: any;

  constructor() {
    // Fast-path: layoutType 'none'/'with-customer-doctor' (print-treatment) không cần chờ logo
    // trigger print sớm sau 200ms khi data đã sẵn sàng
    effect(() => {
      const type = this.layoutType();
      const ready = this.ready();
      if (
        (type === 'none' || type === 'with-customer-doctor') &&
        ready &&
        !this.printTriggered()
      ) {
        clearTimeout(this.printTimeout);
        setTimeout(() => this.triggerPrint(), 200);
      }
    });

    // Fallback print trigger — chỉ bắt đầu đếm SAU KHI context ready (logo URL có thể fetch).
    // Trước đây timer 800ms chạy ngay từ constructor, ăn vào thời gian bootstrap + IndexedDB
    // load → ảnh không kịp load → fallback fire → header hiện skeleton/thiếu logo.
    // Giờ timer chỉ đếm thời gian fetch ảnh thật (preload đã chạy song song qua <link>).
    effect(() => {
      const ready = this.ready();
      const type = this.layoutType();
      if (!ready || type === 'none' || type === 'with-customer-doctor' || this.printTriggered()) return;
      if (this.printTimeout) return; // đã đặt rồi
      this.printTimeout = setTimeout(() => {
        if (!this.printTriggered()) {
          console.log('Fallback print trigger - no logo loaded');
          this.triggerPrint();
        }
      }, 800);
    });
  }

  onLogoLoaded(): void {
    if (this.logoIsLoaded() || this.printTriggered()) return;

    this.logoIsLoaded.set(true);
    clearTimeout(this.printTimeout); // Cancel fallback
    this.triggerPrint();
  }

  private triggerPrint(): void {
    if (this.printTriggered()) return;

    this.printTriggered.set(true);

    // Use requestAnimationFrame for better performance
    requestAnimationFrame(() => {
      setTimeout(() => {
        // Báo parent window (nếu đang trong iframe) hide loading ngay trước khi print dialog mở
        if (window.parent !== window) {
          window.parent.postMessage('print-dialog-opening', '*');
        }
        window.print();
      }, 100);
    });
  }
}
