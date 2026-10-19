import { AfterViewInit, Directive, Input, OnChanges, SimpleChanges } from '@angular/core';
import { EChartsOption } from 'echarts';

import { EchartsDirective } from '@/share/directive/echart/echarts.directive';

import { truncateLabel } from './chart-label.util';

/**
 * Bar Dual Axis Chart Directive
 * Hiển thị 2 metrics với 2 trục Y khác nhau
 *
 * // use
 * <div appBarDualAxisChart [categories]="services" [leftData]="quantities" [rightData]="revenues" class="w-full h-[400px]"></div>
 *
 * // 📁 data
 * services = ['Niềng răng', 'Implant', 'Tẩy trắng', 'Bọc răng sứ'];
 * quantities = [120, 80, 150, 90]; // Số lượng
 * revenues = [500000000, 800000000, 200000000, 600000000]; // Doanh thu (VNĐ)
 *
 * Source: https://echarts.apache.org/examples/en/editor.html?c=multiple-y-axis
 */
@Directive({
  selector: '[appBarDualAxisChart]',
  standalone: true,
})
export class BarDualAxisChartDirective
  extends EchartsDirective
  implements OnChanges, AfterViewInit
{
  @Input({ required: true }) categories!: string[];
  @Input({ required: true }) leftData!: number[]; // Số lượng
  @Input({ required: true }) rightData!: number[]; // Doanh thu
  @Input() leftLabel = 'Số lượng';
  @Input() rightLabel = 'Doanh thu';
  @Input() override renderer: 'canvas' | 'svg' = 'canvas';
  @Input() override theme: 'light' | 'dark' | string = 'light';
  @Input() override useDecal = false;
  @Input() override showToolbox = false;
  /** > 0 ⇒ rút gọn nhãn trục X còn N ký tự; tooltip (trigger axis) vẫn hiện tên danh mục đầy đủ. */
  @Input() categoryLabelMaxLength = 0;

  override ngAfterViewInit() {
    super.ngAfterViewInit();
    if (this.categories?.length && this.leftData?.length && this.rightData?.length) {
      this.updateChart(this.getChartOptions());
    }
  }

  override ngOnChanges(changes: SimpleChanges) {
    super.ngOnChanges(changes);
    if (this.chart && this.categories?.length && this.leftData?.length && this.rightData?.length) {
      this.updateChart(this.getChartOptions());
    }
  }

  private getChartOptions(): EChartsOption {
    if (!this.categories?.length || !this.leftData?.length || !this.rightData?.length) {
      return {};
    }

    return {
      tooltip: {
        trigger: 'axis',
        axisPointer: { type: 'shadow' },
      },
      legend: {
        data: [this.leftLabel, this.rightLabel],
      },
      grid: {
        left: '3%',
        right: '4%',
        bottom: '3%',
        containLabel: true,
      },
      xAxis: {
        type: 'category',
        data: this.categories,
        axisLabel: {
          interval: 0,
          rotate: 30,
          formatter: (value: string) => truncateLabel(value, this.categoryLabelMaxLength),
        },
      },
      yAxis: [
        {
          type: 'value',
          name: this.leftLabel,
          position: 'left',
          axisLabel: {
            formatter: '{value}',
          },
        },
        {
          type: 'value',
          name: this.rightLabel,
          position: 'right',
          axisLabel: {
            formatter: (value: number) => {
              if (value >= 1000000000) return (value / 1000000000).toFixed(1) + 'B';
              if (value >= 1000000) return (value / 1000000).toFixed(0) + 'M';
              if (value >= 1000) return (value / 1000).toFixed(0) + 'K';
              return value.toString();
            },
          },
        },
      ],
      series: [
        {
          name: this.leftLabel,
          type: 'bar',
          data: this.leftData,
          itemStyle: {
            color: '#5470c6',
          },
        },
        {
          name: this.rightLabel,
          type: 'bar',
          yAxisIndex: 1,
          data: this.rightData,
          itemStyle: {
            color: '#91cc75',
          },
        },
      ],
    };
  }
}
