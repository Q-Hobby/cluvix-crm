import { ChangeDetectionStrategy, Component, computed, inject } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { TranslateService } from '@ngx-translate/core';

import { CrmDashboardService } from '@/core/services/crm/crm-dashboard.service';
import { NestedPieChartDirective } from '@/share/directive/echart';

import { sourceNestedPie } from '../shell/chart-transforms';
import { injectChartTheme } from '../shell/chart-theme';
import { createWidgetData, injectWidgetContext } from '../shell/widget-data';
import { isEmptyRows } from '../shell/widget-empty';
import { DashboardWidgetFrame } from '../shell/widget-frame';
import { widgetChartHeight } from '../shell/dashboard-widgets';

type SourceRow = { name: string; value: number; group: string };

/**
 * Nguồn khách hàng (story-08): vòng ngoài top 10 nguồn, vòng trong top 5 nhóm nguồn. Nhãn "khác" qua i18n (bản cũ
 * viết cứng "Nguồn khác"/"Nhóm khác"), đổi theo ngôn ngữ (stream).
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-chart-source-overview',
  imports: [DashboardWidgetFrame, NestedPieChartDirective],
  template: `
    <app-dashboard-widget-frame
      titleKey="dashboard.crm.widgets.source_overview.title"
      infoKey="dashboard.crm.widgets.source_overview.desc"
      [status]="data.status()" skeleton="chart" [bodyHeight]="chartHeight"
      [canHide]="ctx?.canHide() ?? false" (hide)="ctx?.hide()"
      (retry)="data.retry()" (refresh)="data.refresh()">
      @if (pie(); as p) {
        @for (t of [theme()]; track t) {
          <div appNestedPieChart [innerData]="p.inner" [outerData]="p.outer" [theme]="t" [innerLabelMaxLength]="12" [outerLabelMaxLength]="15" class="w-full py-2" [style.height.px]="chartHeight"></div>
        }
      }
    </app-dashboard-widget-frame>
  `,
})
export class ChartSourceOverview {
  private readonly svc = inject(CrmDashboardService);
  protected readonly ctx = injectWidgetContext();
  protected readonly theme = injectChartTheme();
  protected readonly chartHeight = widgetChartHeight('source_overview');
  protected readonly data = createWidgetData<SourceRow[]>('source_overview', (q) => this.svc.overviewChartSource(q), isEmptyRows);
  private readonly labels = toSignal(
    inject(TranslateService).stream(['config.group_source.other_source', 'dashboard.crm.widget.other_groups']),
    { initialValue: {} as Record<string, string> },
  );
  protected readonly pie = computed(() => {
    const rows = this.data.value();
    if (!rows?.length) return null;
    const l = this.labels();
    return sourceNestedPie(rows, {
      otherSources: l['config.group_source.other_source'] ?? '',
      otherGroups: l['dashboard.crm.widget.other_groups'] ?? '',
    });
  });
}
