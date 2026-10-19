import { AfterViewInit, Directive, Input, OnChanges, SimpleChanges } from '@angular/core';
import { EChartsOption } from 'echarts';

import { EchartsDirective } from '@/share/directive/echart/echarts.directive';

/**
 * // use
 * <div appPieChart [data]="pieData" class="w-[400px] h-[400px]"></div>
 *
 * // 📁 data
 * pieData = [
 *   { value: 1048, name: 'Search Engine' },
 *   { value: 735, name: 'Direct' },
 *   { value: 580, name: 'Email' },
 *   { value: 484, name: 'Union Ads' },
 *   { value: 300, name: 'Video Ads' }
 * ];
 */

@Directive({
  selector: '[appPieChart]',
  standalone: true,
})
export class PieChartDirective extends EchartsDirective implements OnChanges, AfterViewInit {
  @Input({ required: true }) data!: any[];
  // Optional, không default — chỉ đổi palette khi caller chủ động truyền (additive-only).
  @Input() colors?: string[];
  @Input() override renderer: 'canvas' | 'svg' = 'canvas';
  @Input() override theme: 'light' | 'dark' | string = 'light';
  @Input() override useDecal = false;
  @Input() override showToolbox = false;

  override ngAfterViewInit(): void {
    super.ngAfterViewInit();
    if (this.data?.length) {
      this.updateChart(this.getChartOptions());
    }
  }

  override ngOnChanges(changes: SimpleChanges): void {
    super.ngOnChanges(changes);
    if (this.chart && this.data?.length) {
      this.updateChart(this.getChartOptions());
    }
  }

  private getChartOptions(): EChartsOption {
    if (!this.data?.length) {
      return {};
    }
    return {
      ...(this.colors ? { color: this.colors } : {}),
      tooltip: { trigger: 'item', formatter: '{b}: {c} ({d}%)' },
      legend: { type: 'scroll', orient: 'horizontal', bottom: 0, left: 'center' },
      series: [{
        type: 'pie',
        radius: '50%',
        data: this.data,
        itemStyle: { borderColor: 'var(--p-content-background)', borderWidth: 2 },
      }],
    };
  }
}
