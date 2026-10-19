import { ChangeDetectionStrategy, Component, inject } from '@angular/core';

import { CrmDashboardService } from '@/core/services/crm/crm-dashboard.service';
import { RoseChartDirective } from '@/share/directive/echart';

import { injectChartTheme } from '../shell/chart-theme';
import { createWidgetData, injectWidgetContext } from '../shell/widget-data';
import { isEmptyRows } from '../shell/widget-empty';
import { DashboardWidgetFrame } from '../shell/widget-frame';
import { widgetChartHeight } from '../shell/dashboard-widgets';

type NameValueRow = { name: string; value: number };

/** Biểu đồ khối `service_type_interest` (epic crm-dashboard-customizable story-08): khung chung + bộ lọc chung; dựng lại khi đổi theme. */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-chart-service-type-interest',
  imports: [DashboardWidgetFrame, RoseChartDirective],
  template: `
    <app-dashboard-widget-frame
      titleKey="dashboard.crm.widgets.service_type_interest.title"
      infoKey="dashboard.crm.widgets.service_type_interest.desc"
      [status]="data.status()" skeleton="chart" [bodyHeight]="chartHeight"
      [canHide]="ctx?.canHide() ?? false" (hide)="ctx?.hide()"
      (retry)="data.retry()" (refresh)="data.refresh()">
      @if (data.value(); as v) {
        @for (t of [theme()]; track t) {
          <div appRoseChart [data]="v" [theme]="t" [labelMaxLength]="20" class="w-full py-2" [style.height.px]="chartHeight"></div>
        }
      }
    </app-dashboard-widget-frame>
  `,
})
export class ChartServiceTypeInterest {
  private readonly svc = inject(CrmDashboardService);
  protected readonly ctx = injectWidgetContext();
  protected readonly theme = injectChartTheme();
  protected readonly chartHeight = widgetChartHeight('service_type_interest');
  protected readonly data = createWidgetData<NameValueRow[]>('service_type_interest', (q) => this.svc.overviewServiceTypeInterest(q), isEmptyRows);
}
