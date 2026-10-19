import { AfterViewInit, Directive, Input, OnChanges, SimpleChanges } from '@angular/core';
import type { EChartsOption } from 'echarts';

import { EchartsDirective } from '@/share/directive/echart/echarts.directive';

/**
 * Sử dụng:
 * <div
 *   appBarStackChart
 *   [xAxisLabels]="['Tháng 1', 'Tháng 2', 'Tháng 3']"
 *   [data]="[
 *     { name: 'Lịch huỷ', data: [1, 2, 3] },
 *     { name: 'Đồng ý', data: [2, 2, 2] },
 *     { name: 'Không đến', data: [0, 2, 1] },
 *     { name: 'Không đồng ý', data: [1, 0, 1] }
 *   ]"
 *   class="w-full h-[400px]"
 * ></div>
 */
@Directive({
  selector: '[appBarStackChart]',
  standalone: true,
})
export class BarStackChartDirective extends EchartsDirective implements OnChanges, AfterViewInit {
  @Input() data: { name: string; data: number[] }[] = [];
  @Input() xAxisLabels: string[] = [];
  @Input() override renderer: 'canvas' | 'svg' = 'canvas';
  @Input() override theme: 'light' | 'dark' | string = 'light';
  @Input() override useDecal = false;
  @Input() override showToolbox = false; // muốn toolbox thì set true

  override ngAfterViewInit(): void {
    super.ngAfterViewInit();
    if (this.data?.length && this.xAxisLabels?.length) {
      this.updateChart(this.getChartOptions());
    }
  }

  override ngOnChanges(changes: SimpleChanges): void {
    super.ngOnChanges(changes);
    if (this.chart && this.data?.length && this.xAxisLabels?.length) {
      this.updateChart(this.getChartOptions());
    }
  }

  private getChartOptions(): EChartsOption {
    if (!this.data?.length || !this.xAxisLabels?.length) {
      return {}; // tránh lỗi khi thiếu data
    }

    return {
      tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
      legend: {},
      grid: { left: '3%', right: '4%', bottom: '3%', containLabel: true },
      xAxis: { type: 'category', data: this.xAxisLabels },
      yAxis: { type: 'value' },
      series: this.data.map((series) => ({
        ...series,
        type: 'bar',
        stack: 'total',
        emphasis: { focus: 'series' },
      })),
    };
  }
}
