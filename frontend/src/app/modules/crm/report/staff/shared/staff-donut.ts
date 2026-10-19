import {ChangeDetectionStrategy, Component, computed, inject, input} from '@angular/core';
import {EChartsOption} from 'echarts';
import {TranslatePipe, TranslateService} from '@ngx-translate/core';

import {EchartsDirective} from '@/share/directive/echart';

/**
 * Biểu đồ donut cho màn tổng hợp theo nhân viên: mỗi lát = 1 tư vấn viên (`item.telesale.fullname`),
 * giá trị = `item[valueKey]` (số lượt hoặc tiền). Dùng chung cho tổng hợp doanh số + hoa hồng.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-staff-donut',
  standalone: true,
  imports: [EchartsDirective, TranslatePipe],
  template: `
    <div class="bg-white rounded-xl border border-gray-200 p-3 h-full flex flex-col">
      <div class="flex items-center gap-2 mb-2">
        <i [class]="icon() + ' text-sm'"></i>
        <span class="font-semibold text-sm text-gray-900">{{ titleKey() | translate }}</span>
      </div>
      @if (chartData().length) {
        <div appEchart [options]="chartOptions()" class="w-full flex-1"></div>
      } @else {
        <div class="flex items-center justify-center flex-1 text-gray-400 text-sm">
          {{ 'common.message.no_data' | translate }}
        </div>
      }
    </div>
  `,
})
export class StaffDonut {
  private translate = inject(TranslateService);

  readonly items = input<any[]>([]);
  /** Field giá trị trong item, vd 'count' | 'totalRevenue' | 'totalCommission'. */
  readonly valueKey = input.required<string>();
  /** i18n key tiêu đề biểu đồ. */
  readonly titleKey = input.required<string>();
  /** i18n key đơn vị hiển thị trong tooltip, vd 'crm.report.staff.unit_times'. */
  readonly unitKey = input<string>('');
  readonly icon = input<string>('pi pi-chart-pie text-blue-500');

  private readonly fmt = new Intl.NumberFormat('vi-VN');

  readonly chartData = computed(() =>
    this.items().reduce<{name: string; value: number}[]>((acc, i) => {
      const v = Number(i?.[this.valueKey()]) || 0;
      if (v > 0) acc.push({name: i?.telesale?.fullname || '—', value: v});
      return acc;
    }, []),
  );

  readonly chartOptions = computed<EChartsOption>(() => {
    const unit = this.unitKey() ? ' ' + this.translate.instant(this.unitKey()) : '';
    return {
      tooltip: {
        trigger: 'item',
        formatter: (params: any) => `${params.name}: ${this.fmt.format(params.value)}${unit} (${params.percent}%)`,
      },
      legend: {
        type: 'scroll',
        orient: 'vertical',
        left: 0,
        top: 'middle',
        width: '38%',
        textStyle: {fontSize: 11, overflow: 'truncate', width: 120} as any,
        pageIconSize: 10,
      },
      series: [
        {
          type: 'pie',
          radius: ['42%', '68%'],
          center: ['68%', '50%'],
          avoidLabelOverlap: false,
          itemStyle: {borderRadius: 8, borderColor: '#fff', borderWidth: 2},
          label: {show: false},
          emphasis: {
            label: {show: true, fontSize: 13, fontWeight: 'bold', formatter: '{d}%'},
            itemStyle: {shadowBlur: 10, shadowOffsetX: 0, shadowColor: 'rgba(0,0,0,0.3)'},
          },
          data: this.chartData(),
        },
      ],
    };
  });
}
