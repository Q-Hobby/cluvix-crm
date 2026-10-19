import { AfterViewInit, Directive, Input, OnChanges, SimpleChanges } from '@angular/core';
import { EChartsOption } from 'echarts';

import { EchartsDirective } from '@/share/directive/echart/echarts.directive';

/**
 * Donut Chart Directive
 * // use
 * <div appDonutChart [data]="donutData" class="w-[400px] h-[400px]"></div>
 *
 * // 📁 data same as pieData
 * donutData = [
 *   { value: 1048, name: 'Search Engine' },
 *   { value: 735, name: 'Direct' },
 *   { value: 580, name: 'Email' },
 *   { value: 484, name: 'Union Ads' },
 *   { value: 300, name: 'Video Ads' }
 * ];
 * Source: https://echarts.apache.org/examples/en/editor.html?c=pie-doughnut
 */
@Directive({
  selector: '[appDonutChart]',
  standalone: true,
})
export class DonutChartDirective extends EchartsDirective implements OnChanges, AfterViewInit {
  @Input({ required: true }) data!: any[];
  // Optional, không default — giữ hành vi cũ (ECharts default palette) cho caller không truyền
  // (vd emr/dashboard/component/patient-flow.ts), chỉ đổi palette khi caller chủ động truyền.
  @Input() colors?: string[];
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
    if (this.chart && this.data?.length && changes['data']) {
      this.updateChart(this.getChartOptions());
    }
  }

  private getChartOptions(): EChartsOption {
    if (!this.data?.length) {
      return {};
    }

    return {
      ...(this.colors ? { color: this.colors } : {}),
      tooltip: {
        trigger: 'item',
        formatter: '{b}: {c} ({d}%)',
      },
      legend: {
        type: 'scroll',
        orient: 'horizontal',
        bottom: 0,
        left: 'center',
      },
      series: [
        {
          type: 'pie',
          radius: ['40%', '70%'],
          avoidLabelOverlap: false,
          itemStyle: {
            borderRadius: 8,
            // dark-safe: viền lẫn vào nền card thay vì trắng cứng (vỡ ở dark theme)
            borderColor: 'var(--p-content-background)',
            borderWidth: 2,
          },
          label: {
            show: false,
            position: 'center',
          },
          emphasis: {
            label: {
              show: true,
              fontSize: 20,
              fontWeight: 'bold',
              formatter: '{b}\n{d}%',
            },
            itemStyle: {
              shadowBlur: 10,
              shadowOffsetX: 0,
              shadowColor: 'rgba(0, 0, 0, 0.5)',
            },
          },
          data: this.data,
        },
      ],
    };
  }
}
