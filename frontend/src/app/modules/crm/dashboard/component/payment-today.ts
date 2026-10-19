import { ChangeDetectionStrategy, Component, inject } from '@angular/core';

import { CrmDashboardService } from '@/core/services/crm/crm-dashboard.service';

import { KpiTile } from '../shell/kpi-tile';
import { createWidgetData, injectWidgetContext } from '../shell/widget-data';
import { DashboardWidgetFrame } from '../shell/widget-frame';

interface PaymentTodayRes { doanhSo: string | number; doanhThu: string | number; thucThu: string | number; congNo: string | number }

/** Doanh thu trong ngày tham chiếu (story-07): nhãn kỳ "Hôm nay"/"Ngày dd/MM" ở header. */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-payment-today',
  imports: [DashboardWidgetFrame, KpiTile],
  template: `
    <app-dashboard-widget-frame
      titleKey="dashboard.crm.widgets.payment_today.title"
      [period]="data.period()" [status]="data.status()" skeleton="kpi"
      [canHide]="ctx?.canHide() ?? false" (hide)="ctx?.hide()"
      (retry)="data.retry()" (refresh)="data.refresh()">
      @if (data.value(); as v) {
        <div class="grid grid-cols-2 gap-3">
          <app-kpi-tile money labelKey="common.label.sales" [value]="v.doanhSo" />
          <app-kpi-tile money labelKey="common.label.revenue" [value]="v.doanhThu" />
          <app-kpi-tile money labelKey="common.label.actual_revenue" [value]="v.thucThu" />
          <app-kpi-tile money labelKey="common.label.expected_revenue" [value]="v.congNo" />
        </div>
      }
    </app-dashboard-widget-frame>
  `,
})
export class PaymentToday {
  private readonly svc = inject(CrmDashboardService);
  protected readonly ctx = injectWidgetContext();
  protected readonly data = createWidgetData<PaymentTodayRes>('payment_today', (q) => this.svc.paymentToday(q));
}
