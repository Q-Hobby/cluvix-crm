import { Component, inject, ChangeDetectionStrategy } from '@angular/core';
import { TranslatePipe } from '@ngx-translate/core';

import { BrandLoader } from '@/share/components/brand-loader/brand-loader';
import { GlobalLoadingService } from '@/share/services/global-loading.service';

@Component({
  selector: 'app-loader',
  standalone: true,
  imports: [BrandLoader, TranslatePipe],
  template: `
    <div
      class="fixed inset-0 z-[9999] bg-black/20 flex items-center justify-center backdrop-blur-sm overlay"
      [class.overlay-visible]="isVisible()"
    >
      <app-brand-loader
        color="#7B3F9E"
        textColor="#fff"
        [text]="(globalLoading.labelKey() ?? 'common.label.loading') | translate"
      />
    </div>
  `,
  changeDetection: ChangeDetectionStrategy.Eager,
  styles: [
    `
      .overlay {
        opacity: 0;
        pointer-events: none;
        transition: opacity 150ms ease;
      }

      .overlay-visible {
        opacity: 1;
        pointer-events: auto;
      }
    `,
  ],
})
export class AppLoader {
  globalLoading = inject(GlobalLoadingService);

  // ✅ Dùng computed để lấy trạng thái hiện tại
  isVisible = this.globalLoading.isLoading;
}
