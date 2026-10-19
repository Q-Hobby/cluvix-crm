import { booleanAttribute, ChangeDetectionStrategy, Component, computed, inject, input } from '@angular/core';
import { TranslatePipe } from '@ngx-translate/core';
import { TooltipModule } from 'primeng/tooltip';

import { formatCurrencyByBusinessProfile } from '@/share/pipes/currency-format/currency-format.pipe';
import { LocaleFormatService } from '@/share/services/locale-format.service';
import { tint } from '@/share/utils/dashboard-tint.util';

import { fullNumber, percentNumber, TrendDirection, trendTone } from './kpi.util';
import { injectWidgetContext } from './widget-data';

/**
 * Thẻ KPI dùng chung (PRD FR22): số chính ĐẦY ĐỦ theo locale thương hiệu (`money` ⇒ kèm ký hiệu tiền theo hồ sơ thương hiệu), chip Δ% có
 * ▲/▼, màu theo trạng thái (xanh tốt / đỏ xấu / xám không đổi). Số tiền từ BE là chuỗi decimal — chỉ
 * HIỂN THỊ, không cộng trừ ở FE.
 */
@Component({
  selector: 'app-kpi-tile',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [TranslatePipe, TooltipModule],
  template: `
    <!-- Nền tint theo nhóm khối, không viền trái (mẫu đã duyệt 2026-10-09). container-type: cỡ số co theo bề rộng THẺ. -->
    <div class="h-full flex flex-col gap-2 p-3 rounded-xl border border-[var(--p-content-border-color)] min-w-0 [container-type:inline-size]"
         [style.background]="accent ? tint(accent, 9) : null" [style.border-color]="accent ? tint(accent, 18) : null">
      <div class="flex items-center gap-1 min-w-0">
        <span class="text-xs font-semibold text-[var(--p-text-color)] truncate" [attr.title]="labelKey() | translate">{{ labelKey() | translate }}</span>
        @if (hintKey()) {
          <!-- <button> (không <i tabindex>) + vùng bấm 24px (root 12px ⇒ w-8) — WCAG 2.2 target size, có role cho trình đọc màn hình. -->
          <button type="button"
                  class="shrink-0 inline-grid place-items-center w-8 h-8 -my-2 rounded-md text-[var(--p-text-muted-color)] hover:bg-[var(--p-content-hover-background)] focus-visible:outline-2 focus-visible:outline-[var(--p-primary-color)]"
                  [pTooltip]="hintKey()! | translate" tooltipPosition="top" [attr.aria-label]="hintKey()! | translate">
            <i class="pi pi-info-circle text-[0.7rem]" aria-hidden="true"></i>
          </button>
        }
        @if (showDelta()) {
          <span class="ml-auto shrink-0 inline-flex items-center gap-0.5 px-1.5 py-0.5 rounded-md text-xs font-semibold tabular-nums"
                [class]="toneClass()">
            @if (direction() === 2) { <i class="pi pi-arrow-up text-[0.6rem]" aria-hidden="true"></i> }
            @if (direction() === 1) { <i class="pi pi-arrow-down text-[0.6rem]" aria-hidden="true"></i> }
            {{ deltaText() }}%
          </span>
        }
      </div>
      <!-- Số ĐẦY ĐỦ (không "tr"/"tỷ"); thẻ hẹp thì cỡ chữ co theo cqi, vẫn hẹp nữa thì xuống dòng — không cắt mất số. -->
      <div class="min-w-0 leading-tight">
        <span class="text-[clamp(1.1rem,9.5cqi,1.75rem)] font-bold text-[var(--p-text-color)] tabular-nums [overflow-wrap:anywhere]">{{ display() }}</span>
        @if (unitKey()) {
          <span class="ml-1 text-xs text-[var(--p-text-muted-color)]">{{ unitKey()! | translate }}</span>
        }
      </div>
    </div>
  `,
})
export class KpiTile {
  readonly labelKey = input.required<string>();
  readonly value = input<number | string | null>(null);
  readonly hintKey = input<string | null>(null);
  /** Đơn vị sau số (i18n key), vd "ngày", "đơn". */
  readonly unitKey = input<string | null>(null);
  /** Số tiền ⇒ định dạng tiền tệ thương hiệu ("320.000.000 ₫"). */
  readonly money = input(false, { transform: booleanAttribute });
  /** % thay đổi so kỳ trước (BE `tyLe`); null ⇒ không hiện chip. */
  readonly deltaPct = input<number | string | null>(null);
  /** BE `tangGiam`: 0 không đổi · 1 giảm · 2 tăng. */
  readonly direction = input<TrendDirection | null>(null);
  /** false khi TĂNG là xấu (hoàn tiền…). */
  readonly upIsGood = input(true);

  private readonly fmt = inject(LocaleFormatService);
  /** Màu nhấn nhóm khối (shell cấp qua context); ngoài shell ⇒ không tint. */
  protected readonly accent = injectWidgetContext()?.accent ?? null;
  protected readonly tint = tint;
  protected readonly display = computed(() => {
    const v = this.value();
    const n = typeof v === 'string' ? Number(v) : v;
    if (this.money() && n !== null && Number.isFinite(n)) return formatCurrencyByBusinessProfile(n);
    return fullNumber(v, this.fmt.currentLocale());
  });
  protected readonly showDelta = computed(() => this.deltaPct() !== null && this.direction() !== null);
  /** BE `tyLe` là chuỗi decimal ("12.5") ⇒ định dạng theo locale (vi-VN "12,5") — QA P5. */
  protected readonly deltaText = computed(() => percentNumber(this.deltaPct(), this.fmt.currentLocale()));
  protected readonly toneClass = computed(() => {
    switch (trendTone(this.direction(), this.upIsGood())) {
      case 'good':
        // Success #10B981 (ui.md); chữ #047857 đạt AA trên nền nhạt, dark dùng #34D399.
        return 'bg-[#10B9811A] text-[#047857] dark:text-[#34D399]';
      case 'bad':
        // Danger #E53935 nền alpha; chữ #C62828 (phương án AA trong ui.md), dark #F87171.
        return 'bg-[#E539351A] text-[#C62828] dark:text-[#F87171]';
      default:
        return 'bg-[var(--p-content-hover-background)] text-[var(--p-text-muted-color)]';
    }
  });
}
