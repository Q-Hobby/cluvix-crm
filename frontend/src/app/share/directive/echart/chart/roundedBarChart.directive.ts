import { AfterViewInit, Directive, Input, OnChanges, SimpleChanges } from '@angular/core';
import { EChartsOption } from 'echarts';
import { EchartsDirective } from '@/share/directive/echart/echarts.directive';
import { CHART_COLORS } from '@/share/directive/echart/chart-colors';

/**
 * Rounded Bar Chart — single-series bar with rounded top + gradient fill.
 * <div appRoundedBarChart [data]="counts" [categories]="months" class="w-full h-[180px]"></div>
 *
 * data = [12, 5, 28, 41, 33, 19]
 * categories = ['2024-12', '2025-01', ...]
 */
@Directive({ selector: '[appRoundedBarChart]', standalone: true })
export class RoundedBarChartDirective extends EchartsDirective implements OnChanges, AfterViewInit {
  @Input({ required: true }) data: number[] = [];
  @Input({ required: true }) categories: string[] = [];
  @Input() color: string = CHART_COLORS.primary;
  @Input() colorEnd: string = '#c4b5fd';
  @Input() override renderer: 'canvas' | 'svg' = 'canvas';

  override ngAfterViewInit() {
    super.ngAfterViewInit();
    if (this.data?.length && this.categories?.length) this.updateChart(this.buildOptions());
  }

  override ngOnChanges(changes: SimpleChanges) {
    super.ngOnChanges(changes);
    if (this.chart && this.data?.length && this.categories?.length) this.updateChart(this.buildOptions());
  }

  private buildOptions(): EChartsOption {
    return {
      tooltip: {
        trigger: 'axis',
        backgroundColor: 'rgba(255,255,255,0.96)',
        borderColor: '#e5e7eb',
        borderWidth: 1,
        textStyle: { color: '#374151', fontSize: 12 },
        axisPointer: { type: 'none' },
      },
      grid: { top: 8, right: 8, bottom: 24, left: 8, containLabel: true },
      xAxis: {
        type: 'category',
        data: this.categories,
        axisLine: { show: false },
        axisTick: { show: false },
        axisLabel: { color: '#9ca3af', fontSize: 11 },
      },
      yAxis: {
        type: 'value',
        axisLabel: { color: '#9ca3af', fontSize: 11 },
        splitLine: { lineStyle: { color: '#f3f4f6', type: 'dashed' } },
        axisLine: { show: false },
        axisTick: { show: false },
      },
      series: [
        {
          type: 'bar',
          data: this.data,
          barMaxWidth: 36,
          itemStyle: {
            borderRadius: [5, 5, 0, 0],
            color: {
              type: 'linear' as const,
              x: 0, y: 0, x2: 0, y2: 1,
              colorStops: [
                { offset: 0, color: this.color },
                { offset: 1, color: this.colorEnd },
              ],
            },
          },
        },
      ],
    };
  }
}
