import { AfterViewInit, Directive, Input, OnChanges, SimpleChanges } from '@angular/core';
import { EChartsOption } from 'echarts';
import { EchartsDirective } from '@/share/directive/echart/echarts.directive';

/**
 * Grouped Bar Chart — multi-series side-by-side bars with rounded tops.
 * <div appGroupedBarChart [data]="series" [categories]="months" class="w-full h-[180px]"></div>
 *
 * data = [{ name: 'Điểm tích lũy', data: [1200, 3400, ...] }, { name: 'Điểm đổi', data: [500, 1200, ...] }]
 * categories = ['2024-12', '2025-01', ...]
 */
@Directive({ selector: '[appGroupedBarChart]', standalone: true })
export class GroupedBarChartDirective extends EchartsDirective implements OnChanges, AfterViewInit {
  @Input({ required: true }) data: { name: string; data: number[] }[] = [];
  @Input({ required: true }) categories: string[] = [];
  @Input() colors: string[] = ['#10b981', '#f59e0b'];
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
        axisPointer: { type: 'shadow' },
      },
      legend: {
        bottom: 0,
        itemWidth: 10,
        itemHeight: 10,
        textStyle: { color: '#6b7280', fontSize: 11 },
        icon: 'roundRect',
      },
      grid: { top: 8, right: 8, bottom: 36, left: 8, containLabel: true },
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
      series: this.data.map((s, i) => ({
        name: s.name,
        type: 'bar' as const,
        data: s.data,
        barMaxWidth: 20,
        barGap: '15%',
        itemStyle: {
          borderRadius: [3, 3, 0, 0],
          color: palette[i % palette.length],
        },
      })),
    };
  }
}
