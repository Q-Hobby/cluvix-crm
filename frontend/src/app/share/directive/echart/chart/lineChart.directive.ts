import { AfterViewInit, Directive, Input, OnChanges, SimpleChanges } from '@angular/core';
import { EChartsOption } from 'echarts';

import { EchartsDirective } from '@/share/directive/echart/echarts.directive';

/**
 * Line Chart Directive
 * // use
 * <div appLineChart [data]="lineData" [categories]="lineCategories" class="w-full h-[300px]"></div>
 *
 * // 📁 data
 * lineCategories = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];
 * lineData = [150, 230, 224, 218, 135, 147, 260];
 */
@Directive({
  selector: '[appLineChart]',
  standalone: true,
})
export class LineChartDirective extends EchartsDirective implements OnChanges, AfterViewInit {
  @Input({ required: true }) data: any;
  @Input({ required: true }) categories!: string[];
  @Input() areaStyle: boolean = false;
  // Override palette series (mặc định ECharts nếu không truyền) — vd DEFAULT_CHART_COLORS.
  @Input() colors?: string[];
  @Input() override renderer: 'canvas' | 'svg' = 'canvas';
  @Input() override theme: 'light' | 'dark' | string = 'light';
  @Input() override useDecal = false;
  @Input() override showToolbox = false;

  override ngAfterViewInit() {
    super.ngAfterViewInit();
    if (this.data?.length && this.categories?.length) {
      this.updateChart(this.getChartOptions());
    }
  }

  override ngOnChanges(changes: SimpleChanges) {
    super.ngOnChanges(changes);
    if (this.chart && this.data?.length && this.categories?.length) {
      this.updateChart(this.getChartOptions());
    }
  }

  private getChartOptions(): EChartsOption {
    if (!this.data?.length) {
      return {}; // trả về empty option tránh lỗi
    }

    return {
      ...(this.colors ? { color: this.colors } : {}),
      tooltip: { trigger: 'axis' },
      legend: {},
      xAxis: { type: 'category', data: this.categories },
      yAxis: { type: 'value' },
      series: this.data.map((series: any) => {
        const baseSeries: any = {
          name: series.name,
          type: 'line',
          stack: 'Total',
          emphasis: {
            focus: 'series',
          },
          data: series.data,
        };

        if (this.areaStyle) {
          baseSeries.areaStyle = {};
        }

        return baseSeries;
      }),
    };
  }
}
