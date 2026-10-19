import { ChangeDetectionStrategy, Component, inject } from '@angular/core';

import { CrmDashboardService } from '@/core/services/crm/crm-dashboard.service';
import { FunnelChartDirective } from '@/share/directive/echart/chart/funnelChart.directive';

import { injectChartTheme } from '../shell/chart-theme';
import { createWidgetData, injectWidgetContext } from '../shell/widget-data';
import { isEmptyRows } from '../shell/widget-empty';
import { DashboardWidgetFrame } from '../shell/widget-frame';
import { widgetChartHeight } from '../shell/dashboard-widgets';

type NameValueRow = { name: string; value: number };

/** Biểu đồ khối `customer_status_overview` (epic crm-dashboard-customizable story-08): khung chung + bộ lọc chung; dựng lại khi đổi theme. */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-customer-status-chart-overview',
  imports: [DashboardWidgetFrame, FunnelChartDirective],
  template: `
    <app-dashboard-widget-frame
      titleKey="dashboard.crm.widgets.customer_status_overview.title"
      infoKey="dashboard.crm.widgets.customer_status_overview.desc"
      [status]="data.status()" skeleton="chart" [bodyHeight]="chartHeight"
      [canHide]="ctx?.canHide() ?? false" (hide)="ctx?.hide()"
      (retry)="data.retry()" (refresh)="data.refresh()">
      @if (data.value(); as v) {
        @for (t of [theme()]; track t) {
          <div appFunnelChart [data]="v" [theme]="t" class="w-full" [style.height.px]="chartHeight"></div>
        }
      }
    </app-dashboard-widget-frame>
  `,
})
export class CustomerStatusChartOverview {
  private readonly svc = inject(CrmDashboardService);
  protected readonly ctx = injectWidgetContext();
  protected readonly theme = injectChartTheme();
  protected readonly chartHeight = widgetChartHeight('customer_status_overview');
  protected readonly data = createWidgetData<NameValueRow[]>('customer_status_overview', (q) => this.svc.statusCustomerOverview(q), isEmptyRows);
}
