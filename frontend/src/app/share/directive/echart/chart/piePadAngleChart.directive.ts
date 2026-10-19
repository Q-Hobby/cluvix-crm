import { AfterViewInit, Directive, Input, OnChanges, OnDestroy, SimpleChanges } from '@angular/core';
import { EChartsOption } from 'echarts';

import { EchartsDirective } from '@/share/directive/echart/echarts.directive';

/**
 * Pie with padAngle Chart Directive
 * // use
 * <div appPiePadAngleChart [data]="pieData" class="w-[400px] h-[400px]"></div>
 *
 * // 📁 data
 * pieData = [
 *   { value: 1048, name: 'Đồng ý' },
 *   { value: 735, name: 'Không đồng ý' },
 *   { value: 580, name: 'Khách cọc' },
 *   { value: 484, name: 'Huỷ' },
 *   { value: 300, name: 'Chưa xử lý' }
 * ];
 * Source: https://echarts.apache.org/examples/en/editor.html?c=pie-padAngle
 */
@Directive({
  selector: '[appPiePadAngleChart]',
  standalone: true,
})
export class PiePadAngleChartDirective
  extends EchartsDirective
  implements OnChanges, AfterViewInit, OnDestroy
{
  @Input({ required: true }) data!: any[];
  @Input() override renderer: 'canvas' | 'svg' = 'canvas';
  @Input() override theme: 'light' | 'dark' | string = 'light';
  @Input() override useDecal = false;
  @Input() override showToolbox = false;

  private breakpointObserver!: ResizeObserver;
  private lastMobile: boolean | null = null;

  override ngAfterViewInit() {
    super.ngAfterViewInit();
    if (this.data?.length) {
      this.updateChart(this.getChartOptions());
    }
    this.breakpointObserver = new ResizeObserver(() => {
      if (!this.data?.length) return;
      const mobile = this.isMobile();
      if (mobile !== this.lastMobile) {
        this.lastMobile = mobile;
        this.updateChart(this.getChartOptions());
      }
    });
    this.breakpointObserver.observe(this.el.nativeElement);
  }

  override ngOnDestroy(): void {
    this.breakpointObserver?.disconnect();
    super.ngOnDestroy();
  }

  override ngOnChanges(changes: SimpleChanges) {
    super.ngOnChanges(changes);
    if (this.chart && this.data?.length && changes['data']) {
      this.lastMobile = null;
      this.updateChart(this.getChartOptions());
    }
  }

  private isMobile(): boolean {
    return (this.el.nativeElement as HTMLElement).offsetWidth < 480;
  }

  private getChartOptions(): EChartsOption {
    if (!this.data?.length) {
      return {};
    }

    const mobile = this.lastMobile ?? this.isMobile();

    return {
      tooltip: {
        trigger: 'item',
        formatter: '{b}: {c} ({d}%)',
      },
      legend: {
        type: 'scroll',
        orient: 'horizontal',
        bottom: 0,
        left: 'center',
        textStyle: mobile ? { fontSize: 11 } : {},
      },
      series: [
        {
          type: 'pie',
          radius: ['40%', '70%'],
          center: ['50%', '45%'],
          padAngle: 5,
          itemStyle: {
            borderRadius: 10,
            borderColor: '#fff',
            borderWidth: 2,
          },
          label: mobile
            ? { show: false }
            : { show: true, formatter: '{b}\n{d}%', fontSize: 12 },
          emphasis: {
            label: {
              show: true,
              fontSize: 14,
              fontWeight: 'bold',
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
