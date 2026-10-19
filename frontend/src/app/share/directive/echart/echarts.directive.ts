import {
  AfterViewInit,
  Directive,
  ElementRef,
  inject,
  Input,
  OnChanges,
  OnDestroy,
  SimpleChanges,
} from '@angular/core';
import { ECharts, EChartsOption, init } from 'echarts';

@Directive({
  selector: '[appEchart]',
  standalone: true,
})
export class EchartsDirective implements AfterViewInit, OnDestroy, OnChanges {
  private static _themeRegistered = false;
  @Input() showToolbox = false;
  @Input() renderer: 'canvas' | 'svg' = 'canvas';
  @Input() theme: 'light' | 'dark' | string = 'light';
  @Input() useDecal = false;
  @Input() options?: EChartsOption; // ✅ Accept options as input

  chart?: ECharts;
  private chartOptions: EChartsOption = {};
  private resizeObserver!: ResizeObserver;
  protected el = inject(ElementRef);

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['options'] && !changes['options'].firstChange && this.chart) {
      this.updateChart(this.options || {});
    }
  }

  ngAfterViewInit(): void {
    const resolvedTheme = this.theme === 'dark' ? 'dark' : undefined;
    this.chart = init(this.el.nativeElement, resolvedTheme, {
      renderer: this.renderer,
    });

    if (this.options) {
      this.updateChart(this.options);
    }

    this.resizeObserver = new ResizeObserver(() => {
      this.chart?.resize();
    });
    this.resizeObserver.observe(this.el.nativeElement);
  }

  ngOnDestroy(): void {
    this.resizeObserver?.disconnect();
    this.chart?.dispose();
  }

  public updateChart(options: EChartsOption) {
    if (!this.chart) return;

    this.chartOptions = { ...options };
    // Theme 'dark' của ECharts tô nền #100C2A (lệch màu card) — để trong suốt cho nền card theo token theme hiện ra
    // (epic crm-dashboard-customizable story-08). Option tự đặt backgroundColor thì giữ nguyên.
    if (this.chartOptions.backgroundColor === undefined) this.chartOptions.backgroundColor = 'transparent';
    this.injectToolbox();
    this.injectDecal();
    this.chart.setOption(this.chartOptions, true);
  }

  private injectToolbox() {
    if (!this.showToolbox) return;
    this.chartOptions.toolbox = {
      feature: {
        saveAsImage: {},
        restore: {},
        dataView: { readOnly: true },
      },
    };
  }

  private injectDecal() {
    if (!this.useDecal || !this.chartOptions?.series) return;

    const decal = {
      symbol: 'rect',
      dashArrayX: [1, 0],
      dashArrayY: [2, 5],
      color: 'rgba(0,0,0,0.05)',
    };

    const seriesList = Array.isArray(this.chartOptions.series)
      ? this.chartOptions.series
      : [this.chartOptions.series];

    seriesList.forEach((s: any) => {
      s.itemStyle = s.itemStyle || {};
      s.itemStyle.decal = decal;
    });
  }
}
