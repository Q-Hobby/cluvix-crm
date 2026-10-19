import { ChangeDetectionStrategy, Component, inject } from '@angular/core';

import { CrmDashboardService } from '@/core/services/crm/crm-dashboard.service';

import { KpiTile } from '../shell/kpi-tile';
import { createWidgetData, injectWidgetContext } from '../shell/widget-data';
import { DashboardWidgetFrame } from '../shell/widget-frame';

interface AdditionalStatsRes {
  reminderTotal: number; reminderDone: number;
}

/**
 * Nhắc việc trong NGÀY tham chiếu. CRM standalone: BE chỉ còn `reminderTotal`/`reminderDone` (CSKH, liên hệ, CTV, coupon
 * đã bỏ cùng module nguồn — `crm/dashboard/handler` CrmAdditionalStats). Bảng reminder không có chi nhánh ⇒ BE bỏ qua
 * `branch_id` (vẫn kiểm quyền) ⇒ ghi chú ở ⓘ để người xem không tưởng số đã lọc theo chi nhánh.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-additional-stats',
  imports: [DashboardWidgetFrame, KpiTile],
  template: `
    <app-dashboard-widget-frame
      titleKey="dashboard.crm.widgets.additional_stats.title"
      infoKey="dashboard.crm.widget.additional_stats_info"
      [period]="data.period()" [status]="data.status()" skeleton="kpi"
      [canHide]="ctx?.canHide() ?? false" (hide)="ctx?.hide()"
      (retry)="data.retry()" (refresh)="data.refresh()">
      @if (data.value(); as v) {
        <div class="grid grid-cols-2 gap-3">
          <app-kpi-tile labelKey="dashboard.crm.additional_stats.reminder_title" [value]="v.reminderTotal" />
          <app-kpi-tile labelKey="dashboard.crm.additional_stats.done" [value]="v.reminderDone" />
        </div>
      }
    </app-dashboard-widget-frame>
  `,
})
export class AdditionalStats {
  private readonly svc = inject(CrmDashboardService);
  protected readonly ctx = injectWidgetContext();
  protected readonly data = createWidgetData<AdditionalStatsRes>('additional_stats', (q) => this.svc.additionalStats(q));
}
