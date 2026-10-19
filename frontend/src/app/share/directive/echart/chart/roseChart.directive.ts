import { AfterViewInit, Directive, Input, OnChanges, OnDestroy, SimpleChanges } from '@angular/core';
import type { DefaultLabelFormatterCallbackParams, EChartsOption } from 'echarts';

import { EchartsDirective } from '@/share/directive/echart/echarts.directive';

import { truncateLabel } from './chart-label.util';

/**
 * Rose Chart Directive (Nightingale Chart)
 * // use
 * <div appRoseChart [data]="roseData" class="w-[400px] h-[400px]"></div>
 *
 * // 📁 data
 * roseData = [
 *   { value: 40, name: 'Rose 1' },
 *   { value: 38, name: 'Rose 2' },
 *   { value: 32, name: 'Rose 3' },
 *   { value: 30, name: 'Rose 4' },
 *   { value: 28, name: 'Rose 5' },
 *   { value: 26, name: 'Rose 6' },
 *   { value: 22, name: 'Rose 7' },
 *   { value: 18, name: 'Rose 8' }
 * ];
 * Source: https://echarts.apache.org/examples/en/editor.html?c=pie-roseType-simple
 */
@Directive({
  selector: '[appRoseChart]',
  standalone: true,
})
export class RoseChartDirective extends EchartsDirective implements OnChanges, AfterViewInit, OnDestroy {
  @Input({ required: true }) data!: any[];
  @Input() override renderer: 'canvas' | 'svg' = 'canvas';
  @Input() override theme: 'light' | 'dark' | string = 'light';
  @Input() override useDecal = false;
  @Input() override showToolbox = false;
  /** > 0 ⇒ rút gọn legend + nhãn lát còn N ký tự; tooltip vẫn hiện tên đầy đủ. */
  @Input() labelMaxLength = 0;

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
        ...(mobile
          ? { type: 'scroll', orient: 'horizontal', bottom: 0, textStyle: { fontSize: 11 } }
          : { type: 'scroll', orient: 'vertical', right: 10, top: 20, bottom: 20 }),
        formatter: (name: string) => truncateLabel(name, this.labelMaxLength),
      },
      series: [
        {
          type: 'pie',
          radius: mobile ? [16, 100] : [20, 140],
          center: mobile ? ['50%', '42%'] : ['40%', '50%'],
          roseType: 'area',
          itemStyle: {
            borderRadius: 8,
          },
          label: { formatter: (p: DefaultLabelFormatterCallbackParams) => truncateLabel(p.name, this.labelMaxLength) },
          data: this.data,
        },
      ],
    };
  }
}
