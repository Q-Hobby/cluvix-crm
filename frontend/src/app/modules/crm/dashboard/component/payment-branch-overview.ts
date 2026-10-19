import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { TranslatePipe } from '@ngx-translate/core';
import { TableModule } from 'primeng/table';

import { CrmDashboardService } from '@/core/services/crm/crm-dashboard.service';
import { CurrencyFormatPipe } from '@/share/pipes/currency-format/currency-format.pipe';

import { createWidgetData, injectWidgetContext } from '../shell/widget-data';
import { isEmptyList } from '../shell/widget-empty';
import { DashboardWidgetFrame } from '../shell/widget-frame';

interface BranchRow {
  id: number; name: string; totalOrderPlan: string | number; totalPayment: string | number; totalDeposit: string | number;
  totalRefundDeposit: string | number; totalRefundServiceUsed: string | number;
}

/** Tổng quan chi nhánh theo kỳ (story-07). Nguồn duy nhất là bộ lọc chung — bỏ 3 input cũ (AC6). */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-payment-branch-overview',
  imports: [DashboardWidgetFrame, TableModule, TranslatePipe, CurrencyFormatPipe],
  template: `
    <app-dashboard-widget-frame
      titleKey="dashboard.crm.widgets.payment_branch_overview.title"
      [status]="data.status()" skeleton="list"
      [canHide]="ctx?.canHide() ?? false" (hide)="ctx?.hide()"
      (retry)="data.retry()" (refresh)="data.refresh()">
      @if (data.value(); as rows) {
        <p-table [value]="rows" [rowHover]="true" [scrollable]="true" dataKey="id" styleClass="text-sm">
          <ng-template #header>
            <tr>
              <th class="w-10 text-center">#</th>
              <th class="min-w-36">{{ 'common.label.branch' | translate }}</th>
              <th class="min-w-28 text-right">{{ 'common.label.revenue' | translate }}</th>
              <th class="min-w-28 text-right">{{ 'common.label.actual_revenue' | translate }}</th>
              <th class="min-w-24 text-right">{{ 'common.label.deposit' | translate }}</th>
              <th class="min-w-24 text-right">{{ 'common.label.refund_deposit' | translate }}</th>
              <th class="min-w-24 text-right">{{ 'common.label.service_refund' | translate }}</th>
            </tr>
          </ng-template>
          <ng-template #body let-row let-i="rowIndex">
            <tr>
              <td class="text-center text-[var(--p-text-muted-color)]">{{ i + 1 }}</td>
              <td class="font-medium text-[var(--p-text-color)]"><span class="block truncate" [attr.title]="row.name">{{ row.name }}</span></td>
              <td class="text-right tabular-nums">{{ row.totalOrderPlan | currencyFormat }}</td>
              <td class="text-right tabular-nums font-semibold">{{ row.totalPayment | currencyFormat }}</td>
              <td class="text-right tabular-nums">{{ row.totalDeposit | currencyFormat }}</td>
              <td class="text-right tabular-nums text-[#C62828] dark:text-[#F87171]">{{ row.totalRefundDeposit | currencyFormat }}</td>
              <td class="text-right tabular-nums text-[#C62828] dark:text-[#F87171]">{{ row.totalRefundServiceUsed | currencyFormat }}</td>
            </tr>
          </ng-template>
        </p-table>
      }
    </app-dashboard-widget-frame>
  `,
})
export class PaymentBranchOverview {
  private readonly svc = inject(CrmDashboardService);
  protected readonly ctx = injectWidgetContext();
  protected readonly data = createWidgetData<BranchRow[]>(
    'payment_branch_overview',
    (q) => this.svc.paymentBranchOverview(q),
    isEmptyList,
  );
}
