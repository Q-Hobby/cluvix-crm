import { ChangeDetectionStrategy, Component, inject } from '@angular/core';

import { CrmDashboardService } from '@/core/services/crm/crm-dashboard.service';

import { KpiTile } from '../shell/kpi-tile';
import type { TrendDirection } from '../shell/kpi.util';
import { createWidgetData, injectWidgetContext } from '../shell/widget-data';
import { DashboardWidgetFrame } from '../shell/widget-frame';

interface Trend { hienTai: string | number; flag?: boolean; tyLe?: string | number; tangGiam?: TrendDirection }
interface PaymentInMonthRes { doanhSo: Trend; doanhThu: string | number; thucThu: Trend; hoanTra: string | number; congNo: string | number }

/** Doanh thu theo kỳ (epic crm-dashboard-customizable story-07): 5 thẻ KPI, Δ so kỳ trước cho doanh số/thực thu. */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-payment-in-month',
  imports: [DashboardWidgetFrame, KpiTile],
  template: `
    <app-dashboard-widget-frame
      titleKey="dashboard.crm.widgets.payment_in_month.title"
      [status]="data.status()" skeleton="kpi"
      [canHide]="ctx?.canHide() ?? false" (hide)="ctx?.hide()"
      (retry)="data.retry()" (refresh)="data.refresh()">
      @if (data.value(); as v) {
        <div class="grid grid-cols-2 lg:grid-cols-5 [@container(min-width:481px)_and_(max-width:800px)]:!grid-cols-3 [@container(max-width:480px)]:!grid-cols-2 gap-3">
          <app-kpi-tile money labelKey="common.label.sales" hintKey="dashboard.crm.payment_in_month.sales_tooltip"
            [value]="v.doanhSo.hienTai" [deltaPct]="v.doanhSo.flag ? (v.doanhSo.tyLe ?? null) : null"
            [direction]="v.doanhSo.flag ? (v.doanhSo.tangGiam ?? null) : null" />
          <app-kpi-tile money labelKey="common.label.revenue" hintKey="common.label.amount_collected_based_on_order_value" [value]="v.doanhThu" />
          <app-kpi-tile money labelKey="common.label.actual_revenue" hintKey="common.label.actual_amount_collected"
            [value]="v.thucThu.hienTai" [deltaPct]="v.thucThu.flag ? (v.thucThu.tyLe ?? null) : null"
            [direction]="v.thucThu.flag ? (v.thucThu.tangGiam ?? null) : null" />
          <app-kpi-tile money labelKey="dashboard.crm.payment_in_month.refund" hintKey="dashboard.crm.payment_in_month.refund_tooltip"
            [value]="v.hoanTra" [upIsGood]="false" />
          <app-kpi-tile money labelKey="common.label.expected_revenue" hintKey="common.label.amount_not_yet_collected_from_customers_based_on" [value]="v.congNo" />
        </div>
      }
    </app-dashboard-widget-frame>
  `,
})
export class PaymentInMonth {
  private readonly svc = inject(CrmDashboardService);
  protected readonly ctx = injectWidgetContext();
  protected readonly data = createWidgetData<PaymentInMonthRes>('payment_in_month', (q) => this.svc.paymentInMonth(q));
}
