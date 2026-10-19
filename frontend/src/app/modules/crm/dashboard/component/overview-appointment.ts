import { ChangeDetectionStrategy, Component, computed, inject } from '@angular/core';

import { CrmDashboardService } from '@/core/services/crm/crm-dashboard.service';

import { KpiTile } from '../shell/kpi-tile';
import { createWidgetData, injectWidgetContext } from '../shell/widget-data';
import { DashboardWidgetFrame } from '../shell/widget-frame';

interface OverviewAppointmentRes { lichHen: number; daDen: number; dongY: number; huy: number; conLaiTrongThang: number; conLai: number }

/**
 * Lịch hẹn tư vấn (story-07): lịch hẹn/đã đến/đồng ý/huỷ tính từ đầu kỳ tới hết ngày tham chiếu; "còn lại trong kỳ"
 * = sau ngày tham chiếu tới cuối kỳ; "tổng còn lại" = mọi lịch sau ngày tham chiếu. Nhãn kỳ ở header.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-overview-appointment',
  imports: [DashboardWidgetFrame, KpiTile],
  template: `
    <app-dashboard-widget-frame
      titleKey="dashboard.crm.widgets.overview_appointment.title"
      [period]="period()" infoKey="dashboard.crm.widget.overview_appointment_info" [status]="data.status()" skeleton="kpi"
      [canHide]="ctx?.canHide() ?? false" (hide)="ctx?.hide()"
      (retry)="data.retry()" (refresh)="data.refresh()">
      @if (data.value(); as v) {
        <div class="grid grid-cols-2 gap-2">
          <app-kpi-tile labelKey="common.label.appointment" [value]="v.lichHen" />
          <app-kpi-tile labelKey="common.status.arrived" [value]="v.daDen" />
          <app-kpi-tile labelKey="common.label.accepted" [value]="v.dongY" />
          <app-kpi-tile labelKey="common.action.cancel" [value]="v.huy" />
          <app-kpi-tile labelKey="dashboard.crm.widget.remaining_in_period" [value]="v.conLaiTrongThang" />
          <app-kpi-tile labelKey="common.label.total_remaining" [value]="v.conLai" />
        </div>
      }
    </app-dashboard-widget-frame>
  `,
})
export class OverviewAppointment {
  private readonly svc = inject(CrmDashboardService);
  protected readonly ctx = injectWidgetContext();
  protected readonly data = createWidgetData<OverviewAppointmentRes>('overview_appointment', (q) => this.svc.overviewAppointment(q));
  /**
   * Các ô lịch hẹn/đã đến/đồng ý/huỷ đếm từ ĐẦU KỲ tới HẾT ngày tham chiếu (BE CrmOverviewAppointment) — nhãn phải nói
   * rõ "từ đầu kỳ đến …", không chỉ "Hôm nay" (đọc nhầm thành số của riêng ngày đó — QA P2).
   */
  protected readonly period = computed(() => {
    const p = this.data.period();
    if (!p) return null;
    return p.params?.['date']
      ? { key: 'dashboard.crm.widget.period_until_day', params: { date: p.params['date'] } }
      : { key: 'dashboard.crm.widget.period_until_today' };
  });
}
