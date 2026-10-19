import { ChangeDetectionStrategy, Component, inject } from '@angular/core';

import { CrmDashboardService } from '@/core/services/crm/crm-dashboard.service';

import { KpiTile } from '../shell/kpi-tile';
import { createWidgetData, injectWidgetContext } from '../shell/widget-data';
import { DashboardWidgetFrame } from '../shell/widget-frame';

interface QuickStatsRes {
  customerMonth: number; customerToday: number; appointmentToday: number; appointmentComing: number; orderNew: number; serviceNew: number;
}

/** Thống kê nhanh (story-07): khách mới "trong kỳ" + các số của ngày tham chiếu (nhãn kỳ ở header). */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-quick-stats',
  imports: [DashboardWidgetFrame, KpiTile],
  template: `
    <app-dashboard-widget-frame
      titleKey="dashboard.crm.widgets.quick_stats.title"
      [period]="data.period()" [status]="data.status()" skeleton="kpi"
      [canHide]="ctx?.canHide() ?? false" (hide)="ctx?.hide()"
      (retry)="data.retry()" (refresh)="data.refresh()">
      @if (data.value(); as v) {
        <div class="grid grid-cols-2 md:grid-cols-3 [@container(max-width:480px)]:!grid-cols-2 gap-3">
          <app-kpi-tile labelKey="dashboard.crm.widget.new_customers_period" [value]="v.customerMonth" />
          <app-kpi-tile labelKey="dashboard.crm.widget.new_customers_day" [value]="v.customerToday" />
          <app-kpi-tile labelKey="dashboard.crm.widget.appointments_created_day" [value]="v.appointmentToday" />
          <app-kpi-tile labelKey="dashboard.crm.widget.appointments_due_day" [value]="v.appointmentComing" />
          <app-kpi-tile labelKey="dashboard.crm.quick_stats.new_orders" [value]="v.orderNew" />
          <app-kpi-tile labelKey="dashboard.crm.quick_stats.new_services" [value]="v.serviceNew" />
        </div>
      }
    </app-dashboard-widget-frame>
  `,
})
export class QuickStats {
  private readonly svc = inject(CrmDashboardService);
  protected readonly ctx = injectWidgetContext();
  protected readonly data = createWidgetData<QuickStatsRes>('quick_stats', (q) => this.svc.quickStats(q));
}
