import { AfterViewInit, Directive, Input, OnChanges, SimpleChanges } from '@angular/core';
import { EChartsOption } from 'echarts';

import { EchartsDirective } from '@/share/directive/echart/echarts.directive';

/**
 * Bar Polar Chart Directive (Radial Bar Chart)
 * // use
 * <div appBarPolarChart [data]="polarData" class="w-[400px] h-[400px]"></div>
 *
 * // 📁 data
 * polarData = [
 *   { value: 40, name: 'Miền Bắc' },
 *   { value: 38, name: 'Miền Trung' },
 *   { value: 32, name: 'Miền Nam' }
 * ];
 * Source: https://echarts.apache.org/examples/en/editor.html?c=bar-polar-label-tangential
 */
@Directive({
  selector: '[appBarPolarChart]',
  standalone: true,
})
export class BarPolarChartDirective extends EchartsDirective implements OnChanges, AfterViewInit {
  @Input({ required: true }) data!: any[];
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
      angleAxis: {
        type: 'category',
        data: this.data.map((item) => item.name),
        startAngle: 90,
      },
      radiusAxis: {},
      polar: {},
      tooltip: {
        trigger: 'item',
        formatter: '{b}: {c}',
      },
      series: [
        {
          type: 'bar',
          data: this.data.map((item) => item.value),
          coordinateSystem: 'polar',
          label: {
            show: true,
            position: 'middle',
            formatter: '{c}',
          },
          itemStyle: {
            borderRadius: 5,
          },
        },
      ],
    };
  }
}
