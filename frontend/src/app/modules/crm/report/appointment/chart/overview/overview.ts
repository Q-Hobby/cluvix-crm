import {ChangeDetectionStrategy, Component, inject, OnInit, signal, ViewChild } from '@angular/core';
import { Table } from 'primeng/table';
import { Observable, map } from 'rxjs';

import { AppointmentModel } from '@/core/models/crm/appointment.model';
import { AppointmentService } from '@/core/services/crm/appointment.service';
import { BaseListV2 } from '@/share/base/base-list-v2';
import { FilterFieldDef } from '@/share/base/list-filter.model';
import { BarStackChartDirective } from '@/share/directive/echart';
import { ListShareImports } from '@/share/import/list-share-imports';
import { ListApiResponse } from '@/share/models/list-api-response';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-appointment-overview',
  templateUrl: './overview.html',
  standalone: true,
  imports: [...ListShareImports, BarStackChartDirective],
})
export class Overview extends BaseListV2<AppointmentModel> implements OnInit {
  modelService = inject(AppointmentService);
  // Nhúng trong chart-scatter.html, không phải route riêng → không được ghi state lên URL trang cha.
  @ViewChild('dt') dt!: Table<AppointmentModel>;
  dataChart = signal<any[]>([]);
  xAxisLabels = signal<string[]>([]);

  constructor() {
    super();
  }

  ngOnInit() {
    this.initListV2();
  }

  // Không có filter panel — bảng chỉ hiển thị dữ liệu tổng hợp theo năm hiện tại.
  protected filterFields(): FilterFieldDef[] {
    return [];
  }

  /**
   * Override `fetchList` chứ KHÔNG override `searchHandler`. Response của
   * `appointmentOverview` là mảng thô (không phải ListApiResponse chuẩn) — bọc lại thành
   * `{items, _meta}` để base xử lý, đồng thời tính dataChart/xAxisLabels cho biểu đồ cạnh bảng.
   */
  protected override fetchList(): Observable<ListApiResponse<AppointmentModel>> {
    const dateTime = this.timezoneService.now();
    const year = dateTime?.getFullYear();
    return this.modelService.appointmentOverview(year).pipe(
      map((response: any) => {
        this.dataChart.set(this.convertToGroupedSeries(response));
        this.xAxisLabels.set(response?.map((d: any) => `Tháng ${d.thang_lh}`));
        return { items: response } as ListApiResponse<AppointmentModel>;
      }),
    );
  }

  convertToGroupedSeries(
    rawData,
    fields = ['lich_hen', 'lich_huy', 'dong_y', 'khong_den', 'khong_dong_y'],
    nameMap = {
      lich_hen: 'Lịch hẹn',
      lich_huy: 'Lịch huỷ',
      dong_y: 'Đồng ý',
      khong_den: 'Không đến',
      khong_dong_y: 'Không đồng ý',
    },
  ) {
    return fields.map((field) => ({
      name: nameMap[field] || field,
      data: rawData.map((item) => item[field]),
    }));
  }
}
