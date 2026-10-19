import { AfterViewInit, Directive, Input, OnChanges, SimpleChanges } from '@angular/core';
import type { EChartsOption } from 'echarts';

import { EchartsDirective } from '@/share/directive/echart/echarts.directive';

/**
 <div appAreaChart [xAxisLabels]="xLabels" [data]="areaData" class="w-[500px] h-[300px]" ></div>
 xLabels = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];

 areaData = [
 { name: 'Email', data: [120, 132, 101, 134, 90, 230, 210], },
 { name: 'Ads', data: [220, 182, 191, 234, 290, 330, 310], },
 ];
 */

@Directive({
  selector: '[appAreaChart]',
  standalone: true,
})
export class AreaChartDirective extends EchartsDirective implements OnChanges, AfterViewInit {
  @Input() data: { name: string; data: number[] }[] = [];
  @Input() xAxisLabels: string[] = [];
  @Input() override renderer: 'canvas' | 'svg' = 'canvas';
  @Input() override theme: 'light' | 'dark' | string = 'light';
  @Input() override useDecal = false;
  @Input() override showToolbox = false;

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
      return {}; // trả về empty option tránh lỗi
    }

    return {
      tooltip: { trigger: 'axis' },
      legend: {},
      xAxis: { type: 'category', data: this.xAxisLabels, boundaryGap: false },
      yAxis: { type: 'value' },
      series: this.data.map((series: any) => ({
        ...series,
        type: 'line',
        areaStyle: {},
        smooth: true,
      })),
    };
  }
}
