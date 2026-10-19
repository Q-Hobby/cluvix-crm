import { ChangeDetectionStrategy, Component, computed, inject } from '@angular/core';
import { TranslatePipe } from '@ngx-translate/core';

import { CrmDashboardService } from '@/core/services/crm/crm-dashboard.service';
import { BarDualAxisChartDirective } from '@/share/directive/echart';

import { serviceStatsSeries } from '../shell/chart-transforms';
import { injectChartTheme } from '../shell/chart-theme';
import { createWidgetData, injectWidgetContext } from '../shell/widget-data';
import { isEmptyServiceStats } from '../shell/widget-empty';
import { DashboardWidgetFrame } from '../shell/widget-frame';
import { widgetChartHeight } from '../shell/dashboard-widgets';

type ServiceRow = { name: string; quantity: number; revenue: number | string };

/** Thống kê dịch vụ theo kỳ (story-08): số lượng + thực thu, trục kép. */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-chart-service-statistics',
  imports: [DashboardWidgetFrame, BarDualAxisChartDirective, TranslatePipe],
  template: `
    <app-dashboard-widget-frame
      titleKey="dashboard.crm.widgets.service_statistics.title"
      infoKey="dashboard.crm.widgets.service_statistics.desc"
      [status]="data.status()" skeleton="chart" [bodyHeight]="chartHeight"
      [canHide]="ctx?.canHide() ?? false" (hide)="ctx?.hide()"
      (retry)="data.retry()" (refresh)="data.refresh()">
      @if (series(); as s) {
        @for (t of [theme()]; track t) {
          <div appBarDualAxisChart
               [categories]="s.categories" [leftData]="s.quantities" [rightData]="s.revenues"
               [leftLabel]="'common.label.quantity' | translate" [rightLabel]="'common.label.revenue' | translate"
               [theme]="t" [categoryLabelMaxLength]="40" class="w-full py-2" [style.height.px]="chartHeight"></div>
        }
      }
    </app-dashboard-widget-frame>
  `,
})
export class ChartServiceStatistics {
  private readonly svc = inject(CrmDashboardService);
  protected readonly ctx = injectWidgetContext();
  protected readonly theme = injectChartTheme();
  protected readonly chartHeight = widgetChartHeight('service_statistics');
  protected readonly data = createWidgetData<ServiceRow[]>('service_statistics', (q) => this.svc.serviceUsedStatistics(q), isEmptyServiceStats);
  protected readonly series = computed(() => {
    const rows = this.data.value();
    return rows?.length ? serviceStatsSeries(rows) : null;
  });
}
