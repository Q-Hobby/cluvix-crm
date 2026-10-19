import { ChangeDetectionStrategy, Component, inject } from '@angular/core';

import { CrmDashboardService } from '@/core/services/crm/crm-dashboard.service';
import { tint } from '@/share/utils/dashboard-tint.util';

import { createWidgetData, injectWidgetContext } from '../shell/widget-data';
import { isEmptyList } from '../shell/widget-empty';
import { DashboardWidgetFrame } from '../shell/widget-frame';

interface NameValue { name: string; value: number }

/**
 * Trạng thái tư vấn trong NGÀY tham chiếu (story-07). Nguồn duy nhất là bộ lọc chung — bỏ 3 input from/to/branchId cũ
 * (không ai truyền, và service cũ còn không gửi chúng — AC6).
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-appointment-accepted-stats',
  imports: [DashboardWidgetFrame],
  template: `
    <app-dashboard-widget-frame
      titleKey="dashboard.crm.widgets.appointment_accepted_stats.title"
      [period]="data.period()" [status]="data.status()" skeleton="list"
      [canHide]="ctx?.canHide() ?? false" (hide)="ctx?.hide()"
      (retry)="data.retry()" (refresh)="data.refresh()">
      @if (data.value(); as items) {
        <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 [@container(max-width:420px)]:!grid-cols-2 [@container(min-width:421px)_and_(max-width:560px)]:!grid-cols-3 [@container(min-width:561px)_and_(max-width:660px)]:!grid-cols-4 gap-2">
          @for (item of items; track item.name) {
            <!-- Cùng kiểu thẻ KPI (kpi-tile): nền tint theo nhóm khối. -->
            <div class="flex flex-col items-center justify-center gap-1 p-3 rounded-xl border border-[var(--p-content-border-color)] min-w-0"
                 [style.background]="accent ? tint(accent, 9) : null" [style.border-color]="accent ? tint(accent, 18) : null">
              <span class="text-2xl font-bold text-[var(--p-text-color)] tabular-nums">{{ item.value }}</span>
              <span class="text-xs font-semibold text-[var(--p-text-color)] text-center leading-tight break-words">{{ item.name }}</span>
            </div>
          }
        </div>
      }
    </app-dashboard-widget-frame>
  `,
})
export class AppointmentAcceptedStats {
  private readonly svc = inject(CrmDashboardService);
  protected readonly ctx = injectWidgetContext();
  protected readonly accent = this.ctx?.accent ?? null;
  protected readonly tint = tint;
  protected readonly data = createWidgetData<NameValue[]>(
    'appointment_accepted_stats',
    (q) => this.svc.appointmentAcceptedStats(q),
    isEmptyList,
  );
}
