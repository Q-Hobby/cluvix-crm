import { AfterViewInit, Directive, Input, OnChanges, SimpleChanges } from '@angular/core';
import { EChartsOption } from 'echarts';
import { EchartsDirective } from '@/share/directive/echart/echarts.directive';
import { DEFAULT_CHART_COLORS } from '@/share/directive/echart/chart-colors';

/**
 * Horizontal Bar Chart Directive
 * // use
 * <div appBarHorizontalChart [data]="hBarData" [categories]="hBarCategories" class="w-full h-[400px]"></div>
 *
 * // 📁 data
 * hBarCategories = ['Paracetamol', 'Amoxicillin', 'Omeprazole'];
 * hBarData = [320, 280, 150];
 * // hoặc multi-series:
 * hBarData = [
 *   { name: 'SL bán', data: [320, 280, 150] },
 *   { name: 'Doanh thu', data: [100, 90, 60] },
 * ];
 * Source: https://echarts.apache.org/examples/en/editor.html?c=bar-y-category
 */
@Directive({
  selector: '[appBarHorizontalChart]',
  standalone: true,
})
export class BarHorizontalChartDirective extends EchartsDirective implements OnChanges, AfterViewInit {
  @Input({ required: true }) data: any;
  @Input({ required: true }) categories!: string[];
  @Input() colors: string[] = DEFAULT_CHART_COLORS;
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
    if (!this.data?.length) return {};

    const isMultiSeries = typeof this.data[0] === 'object' && this.data[0].name;

    const series: any[] = isMultiSeries
      ? this.data.map((s: any, i: number) => ({
          name: s.name,
          type: 'bar',
          data: s.data,
          itemStyle: { color: this.colors[i % this.colors.length], borderRadius: [0, 4, 4, 0] },
        }))
      : [{
          type: 'bar',
          data: this.data,
          itemStyle: { color: this.colors[0], borderRadius: [0, 4, 4, 0] },
          label: { show: true, position: 'right', formatter: '{c}' },
        }];

    return {
      tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
      legend: isMultiSeries ? {} : undefined,
      grid: { left: '3%', right: '10%', bottom: '3%', containLabel: true },
      xAxis: { type: 'value' },
      yAxis: {
        type: 'category',
        data: this.categories,
        inverse: true,
        axisLabel: {
          width: 120,
          overflow: 'truncate',
        },
      },
      series,
    };
  }
}
