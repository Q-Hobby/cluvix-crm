import { AfterViewInit, Directive, Input, OnChanges, OnDestroy, SimpleChanges } from '@angular/core';
import type { DefaultLabelFormatterCallbackParams, EChartsOption } from 'echarts';

import { EchartsDirective } from '@/share/directive/echart/echarts.directive';

import { truncateLabel } from './chart-label.util';

/**
 * Nested Pie Chart - Biểu đồ tròn lồng nhau
 * Source: https://echarts.apache.org/examples/en/editor.html?c=pie-nest
 *
 * Vòng trong: Danh mục tổng/gom nhóm chính
 * Vòng ngoài: Chi tiết/phân tích sâu hơn
 *
 * Usage:
 * <div appNestedPieChart [innerData]="innerData" [outerData]="outerData" class="w-[500px] h-[400px]"></div>
 *
 * innerData = [
 *   { name: 'Online', value: 335 },
 *   { name: 'Offline', value: 234 },
 *   { name: 'Giới thiệu', value: 135 }
 * ];
 *
 * outerData = [
 *   { name: 'Facebook', value: 135 },
 *   { name: 'Google', value: 100 },
 *   { name: 'Zalo', value: 100 },
 *   { name: 'Walk-in', value: 134 },
 *   { name: 'Event', value: 100 },
 *   { name: 'Bạn bè', value: 135 }
 * ];
 */

@Directive({
  selector: '[appNestedPieChart]',
  standalone: true,
})
export class NestedPieChartDirective extends EchartsDirective implements OnChanges, AfterViewInit, OnDestroy {
  @Input() innerData: { name: string; value: number }[] = [];
  @Input() outerData: { name: string; value: number }[] = [];
  @Input() override renderer: 'canvas' | 'svg' = 'canvas';
  @Input() override theme: 'light' | 'dark' | string = 'light';
  /** > 0 ⇒ rút gọn nhãn vòng trong / vòng ngoài + legend (theo vòng ngoài); tooltip vẫn hiện tên đầy đủ. */
  @Input() innerLabelMaxLength = 0;
  @Input() outerLabelMaxLength = 0;
  @Input() override useDecal = false;
  @Input() override showToolbox = false;

  private breakpointObserver!: ResizeObserver;
  private lastMobile: boolean | null = null;

  override ngAfterViewInit(): void {
    super.ngAfterViewInit();
    if (this.innerData?.length && this.outerData?.length) {
      this.updateChart(this.getChartOptions());
    }
    this.breakpointObserver = new ResizeObserver(() => {
      if (!this.innerData?.length || !this.outerData?.length) return;
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

  override ngOnChanges(changes: SimpleChanges): void {
    super.ngOnChanges(changes);
    if ((changes['innerData'] || changes['outerData']) && this.chart) {
      this.lastMobile = null;
      this.updateChart(this.getChartOptions());
    }
  }

  private isMobile(): boolean {
    return (this.el.nativeElement as HTMLElement).offsetWidth < 480;
  }

  private getChartOptions(): EChartsOption {
    if (!this.innerData?.length || !this.outerData?.length) {
      return {};
    }

    const mobile = this.lastMobile ?? this.isMobile();

    return {
      tooltip: {
        trigger: 'item',
        formatter: '{b}: {c} ({d}%)',
      },
      legend: {
        ...(mobile
          ? { orient: 'horizontal', bottom: 0, type: 'scroll', textStyle: { fontSize: 11 } }
          : { orient: 'vertical', left: 'left' }),
        formatter: (name: string) => truncateLabel(name, this.outerLabelMaxLength),
      },
      series: [
        {
          name: 'Tổng quan',
          type: 'pie',
          selectedMode: 'single',
          radius: [0, '30%'],
          center: mobile ? ['50%', '42%'] : ['50%', '50%'],
          label: {
            position: 'inner',
            fontSize: mobile ? 10 : 14,
            formatter: (p: DefaultLabelFormatterCallbackParams) => truncateLabel(p.name, this.innerLabelMaxLength),
          },
          labelLine: {
            show: false,
          },
          data: this.innerData,
        },
        {
          name: 'Chi tiết',
          type: 'pie',
          radius: ['45%', '60%'],
          center: mobile ? ['50%', '42%'] : ['50%', '50%'],
          label: mobile
            ? { show: false }
            : {
                formatter: (p: DefaultLabelFormatterCallbackParams) =>
                  `${truncateLabel(p.name, this.outerLabelMaxLength)}: ${p.value} (${p.percent}%)`,
              },
          data: this.outerData,
        },
      ],
    };
  }
}
