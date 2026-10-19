import { AfterViewInit, Directive, Input, OnChanges, SimpleChanges } from '@angular/core';
import { EChartsOption } from 'echarts';
import { EchartsDirective } from '@/share/directive/echart/echarts.directive';
import { CHART_COLORS } from '@/share/directive/echart/chart-colors';

/**
 * Gradient Area Chart — multi-series smooth area with gradient fill.
 * <div appGradientAreaChart [data]="series" [categories]="months" class="w-full h-[260px]"></div>
 *
 * data = [{ name: 'Revenue', data: [120, 200, ...] }, { name: 'Commission', data: [30, 50, ...] }]
 * categories = ['2024-12', '2025-01', ...]
 */
@Directive({ selector: '[appGradientAreaChart]', standalone: true })
export class GradientAreaChartDirective extends EchartsDirective implements OnChanges, AfterViewInit {
  @Input({ required: true }) data: { name: string; data: number[] }[] = [];
  @Input({ required: true }) categories: string[] = [];
  @Input() colors: string[] = [CHART_COLORS.primary, '#0ea5e9'];
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
    const palette = this.colors;
    return {
      tooltip: {
        trigger: 'axis',
        backgroundColor: 'rgba(255,255,255,0.96)',
        borderColor: '#e5e7eb',
        borderWidth: 1,
        textStyle: { color: '#374151', fontSize: 12 },
        axisPointer: { lineStyle: { color: '#e5e7eb' } },
      },
      legend: {
        bottom: 0,
        itemWidth: 12,
        itemHeight: 8,
        textStyle: { color: '#6b7280', fontSize: 11 },
        icon: 'roundRect',
      },
      grid: { top: 12, right: 16, bottom: 40, left: 16, containLabel: true },
      xAxis: {
        type: 'category',
        data: this.categories,
        boundaryGap: false,
        axisLine: { lineStyle: { color: '#f3f4f6' } },
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
      series: this.data.map((s, i) => {
        const color = palette[i % palette.length];
        return {
          name: s.name,
          type: 'line' as const,
          smooth: true,
          symbol: 'none',
          lineStyle: { width: 2, color },
          itemStyle: { color },
          areaStyle: {
            color: {
              type: 'linear' as const,
              x: 0, y: 0, x2: 0, y2: 1,
              colorStops: [
                { offset: 0, color: color + '55' },
                { offset: 1, color: color + '08' },
              ],
            },
          },
          data: s.data,
        };
      }),
    };
  }
}
