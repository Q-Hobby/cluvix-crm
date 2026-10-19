import { ChangeDetectionStrategy, Component, inject } from '@angular/core';

import { CrmDashboardService } from '@/core/services/crm/crm-dashboard.service';
import { BarPolarChartDirective } from '@/share/directive/echart';

import { injectChartTheme } from '../shell/chart-theme';
import { createWidgetData, injectWidgetContext } from '../shell/widget-data';
import { isEmptyRows } from '../shell/widget-empty';
import { DashboardWidgetFrame } from '../shell/widget-frame';
import { widgetChartHeight } from '../shell/dashboard-widgets';

type NameValueRow = { name: string; value: number };

/** Biểu đồ khối `customer_region` (epic crm-dashboard-customizable story-08): khung chung + bộ lọc chung; dựng lại khi đổi theme. */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-chart-customer-region',
  imports: [DashboardWidgetFrame, BarPolarChartDirective],
  template: `
    <app-dashboard-widget-frame
      titleKey="dashboard.crm.widgets.customer_region.title"
      infoKey="dashboard.crm.widgets.customer_region.desc"
      [status]="data.status()" skeleton="chart" [bodyHeight]="chartHeight"
      [canHide]="ctx?.canHide() ?? false" (hide)="ctx?.hide()"
      (retry)="data.retry()" (refresh)="data.refresh()">
      @if (data.value(); as v) {
        @for (t of [theme()]; track t) {
          <div appBarPolarChart [data]="v" [theme]="t" class="w-full py-2" [style.height.px]="chartHeight"></div>
        }
      }
    </app-dashboard-widget-frame>
  `,
})
export class ChartCustomerRegion {
  private readonly svc = inject(CrmDashboardService);
  protected readonly ctx = injectWidgetContext();
  protected readonly theme = injectChartTheme();
  protected readonly chartHeight = widgetChartHeight('customer_region');
  protected readonly data = createWidgetData<NameValueRow[]>('customer_region', (q) => this.svc.reportCustomerByRegion(q), isEmptyRows);
}
