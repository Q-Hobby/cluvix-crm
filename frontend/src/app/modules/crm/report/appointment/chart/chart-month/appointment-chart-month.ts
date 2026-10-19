import {ChangeDetectionStrategy, Component, inject, OnInit } from '@angular/core';

import { DashboardService } from '@/core/services/dashboard.service';
import { AreaChartDirective } from '@/share/directive/echart/chart/areaChart.directive';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-chart-month',
  templateUrl: './appointment-chart-month.html',
  styles: [],
  standalone: true,
  imports: [AreaChartDirective],
})
export class AppointmentChartMonth implements OnInit {
  modelService = inject(DashboardService);
  xLabels: any;
  areaData!: [{ name: string; data: any[] }, { name: string; data: any[] }];
  dataLastMonth!: { value: any }[];
  dataThisMonth!: { value: any }[];

  constructor() {}

  ngOnInit(): void {
    this.initChart();
  }

  initChart() {
    this.xLabels = Array.from({ length: 31 }, (_, i) => (i + 1).toString());

    this.modelService.compareappointment().subscribe((response: any) => {
      const lastMonthMap = new Map<number, number>(
        response.dataLastMonth.map((v: any) => [v.x, v.y]),
      );
      const thisMonthMap = new Map<number, number>(
        response.dataThisMonth.map((v: any) => [v.x, v.y]),
      );

      this.dataLastMonth = this.xLabels.map((dayStr) => {
        const day = parseInt(dayStr, 10);
        return lastMonthMap.get(day) ?? 0;
      });

      this.dataThisMonth = this.xLabels.map((dayStr) => {
        const day = parseInt(dayStr, 10);
        return thisMonthMap.get(day) ?? 0;
      });

      this.areaData = [
        { name: 'Tháng trước', data: this.dataLastMonth },
        { name: 'Tháng này', data: this.dataThisMonth },
      ];
    });
  }
}
