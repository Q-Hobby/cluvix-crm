import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { TranslatePipe } from '@ngx-translate/core';
import { ButtonModule } from 'primeng/button';

import { tint } from '@/share/utils/dashboard-tint.util';

import { CRM_DASHBOARD_WIDGET_MAP, type DashboardWidgetDef, WIDGET_CATEGORY_ACCENT } from './dashboard-widgets';
import { DASHBOARD_LAYOUT_PRESETS, type DashboardPresetId, presetLayout } from './layout-presets';
import { zoneItems } from './layout.util';

interface ThumbBlock {
  key: string;
  span: number;
  height: number;
  color: string;
}

interface PresetCard {
  id: DashboardPresetId;
  labelKey: string;
  descKey: string;
  icon: string;
  top: ThumbBlock[];
  main: ThumbBlock[];
  side: ThumbBlock[];
}

/** Chiều cao khối trong hình thu nhỏ (px) — tỉ lệ theo khối thật: biểu đồ theo `chartHeight`, KPI/danh sách thấp. */
function thumbBlocks(registry: readonly DashboardWidgetDef[], id: DashboardPresetId, zone: 'top' | 'main' | 'side'): ThumbBlock[] {
  return zoneItems(presetLayout(id, registry), zone).map((it) => {
    const def = CRM_DASHBOARD_WIDGET_MAP.get(it.key)!;
    return {
      key: it.key,
      span: zone === 'main' && it.size === 'half' ? 1 : 2,
      height: def.skeleton === 'chart' ? Math.round((def.chartHeight ?? 300) / 34) : 9,
      color: tint(WIDGET_CATEGORY_ACCENT[def.category], 55),
    };
  });
}

/** Thẻ mẫu dựng trên registry ĐÃ LỌC QUYỀN ⇒ hình thu nhỏ không có khối user không được thấy. */
export function presetCards(registry: readonly DashboardWidgetDef[]): PresetCard[] {
  return DASHBOARD_LAYOUT_PRESETS.map((p) => ({
    id: p.id,
    labelKey: p.labelKey,
    descKey: `dashboard.crm.layout.preset_desc.${p.id}`,
    icon: p.icon,
    top: thumbBlocks(registry, p.id, 'top'),
    main: thumbBlocks(registry, p.id, 'main'),
    side: thumbBlocks(registry, p.id, 'side'),
  }));
}

/**
 * "Chọn bố cục khởi đầu" — hiện khi user CHƯA có bố cục riêng và thương hiệu CHƯA đặt mặc định (bố cục hệ thống).
 * 4 thẻ mẫu (đủ mọi khối user được thấy) kèm hình thu nhỏ dựng từ chính `presetLayout` ⇒ đổi mẫu là hình đổi theo, không ảnh tĩnh.
 * Chọn ⇒ cha lưu thành bố cục riêng; "Giữ bố cục hiện tại" ⇒ cha ghi nhớ đã bỏ qua (thiết bị).
 */
@Component({
  selector: 'app-dashboard-preset-chooser',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [TranslatePipe, ButtonModule],
  template: `
    <section class="mt-4 p-4 rounded-xl border border-[var(--p-content-border-color)] bg-[var(--p-content-background)] shadow-sm"
             aria-labelledby="dash-preset-chooser-title">
      <div class="flex flex-wrap items-start gap-3">
        <div class="flex-1 min-w-0">
          <h2 id="dash-preset-chooser-title" class="m-0 text-lg font-bold text-[var(--p-text-color)]">{{ 'dashboard.crm.layout.chooser_title' | translate }}</h2>
          <p class="m-0 mt-1 text-sm text-[var(--p-text-muted-color)]">{{ 'dashboard.crm.layout.chooser_desc' | translate }}</p>
        </div>
        <p-button [text]="true" size="small" severity="secondary" [label]="'dashboard.crm.layout.chooser_keep' | translate"
                  [disabled]="busy()" (onClick)="dismiss.emit()" />
      </div>
      <div class="mt-4 grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-3">
        @for (c of cards(); track c.id) {
          <article class="flex flex-col gap-3 p-3 rounded-xl border border-[var(--p-content-border-color)] hover:border-[var(--p-primary-color)] transition-colors">
            <!-- Hình thu nhỏ: vùng trên + cột chính (2 cột con, khối ½ chiếm 1) + cột phụ; màu theo nhóm khối. -->
            <div class="flex flex-col gap-1 p-2 rounded-lg bg-[var(--p-content-hover-background)] h-44 overflow-hidden" aria-hidden="true">
              @for (b of c.top; track b.key) {
                <div class="rounded-sm shrink-0" [style.height.px]="b.height" [style.background]="b.color"></div>
              }
              <div class="flex gap-1 min-h-0">
                <div class="grid grid-cols-2 gap-1 content-start [flex:2_1_0]">
                  @for (b of c.main; track b.key) {
                    <div class="rounded-sm" [class.col-span-2]="b.span === 2" [style.height.px]="b.height" [style.background]="b.color"></div>
                  }
                </div>
                <div class="flex flex-col gap-1 [flex:1_1_0]">
                  @for (b of c.side; track b.key) {
                    <div class="rounded-sm shrink-0" [style.height.px]="b.height" [style.background]="b.color"></div>
                  }
                </div>
              </div>
            </div>
            <div class="flex items-start gap-2 min-w-0">
              <i [class]="c.icon + ' mt-0.5 text-[var(--p-primary-color)]'" aria-hidden="true"></i>
              <div class="min-w-0">
                <h3 class="m-0 text-base font-semibold text-[var(--p-text-color)]">{{ c.labelKey | translate }}</h3>
                <p class="m-0 mt-1 text-xs text-[var(--p-text-muted-color)]">{{ c.descKey | translate }}</p>
              </div>
            </div>
            <p-button class="mt-auto" styleClass="w-full" size="small" [outlined]="true" [label]="'dashboard.crm.layout.chooser_use' | translate"
                      [disabled]="busy()" [ariaLabel]="('dashboard.crm.layout.chooser_use' | translate) + ': ' + (c.labelKey | translate)"
                      (onClick)="pick.emit(c.id)" />
          </article>
        }
      </div>
    </section>
  `,
})
export class DashboardPresetChooser {
  /** Registry đã lọc quyền (`DashboardLayoutStore.registry`). */
  readonly registry = input.required<readonly DashboardWidgetDef[]>();
  readonly busy = input(false);
  readonly pick = output<DashboardPresetId>();
  readonly dismiss = output<void>();

  protected readonly cards = computed(() => presetCards(this.registry()));
}
