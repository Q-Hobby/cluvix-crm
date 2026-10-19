import {ChangeDetectionStrategy, Component, inject, OnInit } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { finalize } from 'rxjs';

import { AppointmentChartMonth } from '@/modules/crm/report/appointment/chart/chart-month/appointment-chart-month';
import { Overview } from '@/modules/crm/report/appointment/chart/overview/overview';
import { AppointmentModel } from '@/core/models/crm/appointment.model';
import { AppointmentService } from '@/core/services/crm/appointment.service';
import { BaseList } from '@/share/base/base-list';
import { LineChartDirective } from '@/share/directive/echart/chart/lineChart.directive';
import { ListShareImports } from '@/share/import/list-share-imports';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-chart-scatter',
  templateUrl: './appointment-chart-scatter.html',
  standalone: true,
  imports: [
    ...ListShareImports,
    AppointmentChartMonth,
    LineChartDirective,
    Overview,
    FillHeightDirective,
  ],
})
export class AppointmentChartScatter extends BaseList<AppointmentModel> implements OnInit {
  modelService = inject(AppointmentService);
  options!: number;
  lineCategories: any;
  lineData: any;

  constructor() {
    super();
  }

  ngOnInit() {
    this.defaultSearch.set({
      '_time[gte]': this.timezoneService.monthStartUnix(),
      '_time[lte]': this.timezoneService.todayEndUnix(),
    });
    void this.initFilter();
    this.initChartSplineArea();
    this.initBaseList();
  }

  /**
   * Biểu đồ lịch hẹn
   */
  initChartSplineArea() {
    const params = {
      ...this.defaultSearch(),
    } as any;
    this.modelService
      .getAppointmentChartStatus({
        params,
      })
      .pipe(
        finalize(() => {
          this.loading.set(false);
        }),
      )
      .subscribe((response: any) => {
        const now = this.timezoneService.now();
        const year = now.getFullYear();
        const month = now.getMonth() + 1; // Tháng hiện tại (1-12)

        const daysInMonth = new Date(year, month, 0).getDate();

        const xAxisLabels = Array.from(
          { length: daysInMonth },
          (_, i) =>
            `${(i + 1).toString().padStart(2, '0')}-${month.toString().padStart(2, '0')}-${year}`,
        );

        this.lineCategories = xAxisLabels;

        // response là dữ liệu bạn lấy từ API
        const { lichHen = [], khachDen = [], khachLam = [] } = response;

        // Map nhanh để lookup
        const createMap = (arr: any[]) => {
          const map = new Map<string, number>();
          arr.forEach((item) => {
            map.set(item.x, item.y);
          });
          return map;
        };

        const lichHenMap = createMap(lichHen);
        const khachDenMap = createMap(khachDen);
        const khachLamMap = createMap(khachLam);

        // Build final array
        this.lineData = [
          {
            name: 'lichHen',
            data: xAxisLabels.map((date) => lichHenMap.get(date) ?? 0),
          },
          {
            name: 'khachDen',
            data: xAxisLabels.map((date) => khachDenMap.get(date) ?? 0),
          },
          {
            name: 'khachLam',
            data: xAxisLabels.map((date) => khachLamMap.get(date) ?? 0),
          },
        ];
      });
  }

  /**
   * Bộ lọc của biểu đồ lịch hẹn
   */
  initFilter() {
    this.rfFilterModel = this.fb.group({
      _time: this.fb.control([
        this.timezoneService.monthStart(),
        this.timezoneService.todayStart(),
      ]),
    });

    this.setDefaultDate();

    this.rfFilterModel.controls['_time'].valueChanges.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((value: any) => {
      if (!value) {
        return null;
      }
      const filterValue = [...value];
      if (!filterValue[1]) {
        filterValue[1] = filterValue[0];
      }
      filterValue[0] = this.timezoneService.dateHmsToUnix(filterValue[0], 0, 0, 0);
      filterValue[1] = this.timezoneService.dateHmsToUnix(filterValue[1], 23, 59, 59);
      this.defaultSearch.set({
        '_time[gte]': filterValue[0],
        '_time[lte]': filterValue[1],
      });
      this.initChartSplineArea();
    });
  }

  onQuickSelectTime({ start, end }: { start: Date; end: Date }) {
    this.rfFilterModel.patchValue({ _time: [start, end] });
  }

  /**
   * Set thời gian default hiển thị biểu đồ lịch hẹn
   */
  setDefaultDate() {
    this.rfFilterModel.patchValue({
      _time: [this.timezoneService.monthStart(), this.timezoneService.todayStart()],
    });
  }

  /**
   * Tại lại biểu đồ lịch hẹn
   */
  override resetPage() {
    this.initChartSplineArea();
  }

  clearStartTime() {
    this.defaultSearch.set({});
    this.rfFilterModel.patchValue({
      _time: null,
    });
    this.initChartSplineArea();
  }
}
