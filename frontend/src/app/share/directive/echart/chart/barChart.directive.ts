// ✅ Directive Bar Chart
import { AfterViewInit, Directive, Input, OnChanges, SimpleChanges } from '@angular/core';
import { EChartsOption } from 'echarts';

import { EchartsDirective } from '@/share/directive/echart/echarts.directive';

/**
 * Bar Chart Directive
 * // use
 * <div appBarChart [data]="barData" [categories]="barCategories" class="w-[500px] h-[300px]"></div>
 *
 * // 📁 data
 * barCategories = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];
 * barData = [120, 200, 150, 80, 70, 110, 130];
 * Source: https://echarts.apache.org/examples/en/editor.html?c=dataset-encode0
 */

@Directive({
  selector: '[appBarChart]',
  standalone: true,
})
export class BarChartDirective extends EchartsDirective implements OnChanges, AfterViewInit {
  @Input({ required: true }) data!: any[];
  @Input({ required: true }) categories!: string[];
  @Input() visualMap: boolean = false;
  @Input() override renderer: 'canvas' | 'svg' = 'canvas';
  @Input() override theme: 'light' | 'dark' | string = 'light';
  @Input() override useDecal = false;
  @Input() override showToolbox = false;

  override ngAfterViewInit() {
    super.ngAfterViewInit();
    if (this.data?.length) {
      this.updateChart(this.getChartOptions());
    }
  }

  override ngOnChanges(changes: SimpleChanges) {
    super.ngOnChanges(changes);
    if (
      this.chart &&
      this.data?.length &&
      (changes['data'] || changes['categories'] || changes['visualMap'])
    ) {
      this.updateChart(this.getChartOptions());
    }
  }

  private getChartOptions(): EChartsOption {
    if (!this.data?.length) {
      return {};
    }

    const baseTooltip = {
      trigger: 'axis' as const,
      axisPointer: { type: 'shadow' as const },
    };

    if (!this.visualMap) {
      return {
        tooltip: baseTooltip,
        xAxis: {
          type: 'category',
          data: this.categories,
          axisLabel: { interval: 0, rotate: 30 },
        },
        yAxis: {
          type: 'value',
          axisLabel: { formatter: '{value}%' },
        },
        series: [
          {
            type: 'bar',
            data: this.data,
            itemStyle: {
              color: '#5470c6',
            },
          },
        ],
      };
    } else {
      return {
        dataset: { source: this.data },
        grid: { containLabel: true },
        tooltip: baseTooltip,
        xAxis: {
          name: 'Tỷ lệ',
          axisLabel: { formatter: '{value}%' },
        },
        yAxis: { type: 'category' },
        visualMap: {
          orient: 'horizontal',
          left: 'center',
          min: 10,
          max: 100,
          text: ['Cao nhất', 'Thấp nhất'],
          dimension: 0,
          inRange: {
            color: ['#65B581', '#FFCE34', '#FD665F'],
          },
        },
        series: [
          {
            type: 'bar',
            encode: {
              x: 'Tỷ lệ',
              y: 'Online',
            },
          },
        ],
      };
    }
  }
}
