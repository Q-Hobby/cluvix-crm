import {ChangeDetectionStrategy, Component, inject, OnInit, signal, ViewChild} from '@angular/core';
import {Table} from 'primeng/table';
import {finalize, map, Observable} from 'rxjs';

import {BaseListV2} from '@/share/base/base-list-v2';
import {FilterFieldDef} from '@/share/base/list-filter.model';
import {ListApiResponse} from '@/share/models/list-api-response';
import {ListShareImports} from '@/share/import/list-share-imports';
import {PERMISSION_SELECT} from '@/define';
import {ReportStaffCrmService} from '@/core/services/crm/report-staff-crm.service';
import {copyReportSummary} from '@/share/utils/report-summary-export.util';
import {StaffDonut} from '@/modules/crm/report/staff/shared/staff-donut';

/**
 * Tổng hợp doanh số theo tư vấn viên (GROUP BY `order_revenue_telesale.revenue_id`).
 * BE: GET /crm/report/staff/revenue-telesale/summary. Port từ ERP accountant/report/revenue-telesale-summary.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-crm-report-revenue-telesale-summary',
  templateUrl: './revenue-telesale-summary.html',
  imports: [...ListShareImports, StaffDonut],
  standalone: true,
})
export class CrmRevenueTelesaleSummary extends BaseListV2<any> implements OnInit {
  modelService = inject(ReportStaffCrmService);
  @ViewChild('dt') dt!: Table;

  totalRevenue = signal<number>(0);

  protected override URL_STATE_PREFIX = 'srs';

  ngOnInit() {
    this.initListV2();
  }

  protected filterFields(): FilterFieldDef[] {
    return [
      {name: 'revenue_id', kind: 'in'},
      {name: 'branch_id', kind: 'in'},
      {
        name: 'confirm_time',
        kind: 'dateRange',
        default: () => [this.timezoneService.monthStart(), this.timezoneService.todayEnd()],
      },
    ];
  }

  protected override fetchList(): Observable<ListApiResponse<any>> {
    return this.modelService.revenueTelesaleSummary({params: this.searchParams()}).pipe(
      map((response: any) => {
        this.totalRevenue.set(Number(response?.totalRevenue) || 0);
        return {items: response?.items ?? [], _meta: response?._meta} as ListApiResponse<any>;
      }),
    );
  }

  onQuickSelect({start, end}: {start: Date; end: Date}) {
    this.rfFilterModel.patchValue({confirm_time: [start, end]});
  }

  clearConfirmTime() {
    this.setFilterValue('confirm_time', null);
  }

  /** Copy bảng (trang đang xem) ra clipboard — client-side. */
  copyData() {
    copyReportSummary(
      {
        items: this.models(),
        cols: [
          {value: 'count', labelKey: 'crm.report.staff.count'},
          {value: 'totalRevenue', labelKey: 'crm.report.staff.revenue'},
        ],
        totals: {totalRevenue: this.totalRevenue()},
        firstColHeaderKey: 'common.label.consultant',
        titleKey: 'crm.report.staff.revenue_telesale_summary',
        filenameBase: 'doanh-so-tu-van-vien',
        permission: PERMISSION_SELECT.permissionManageCrmReport,
        labelFormatter: (_v: any, item: any) => item?.telesale?.fullname || '—',
      },
      {translate: this.translate, noti: this.noti, permissionsService: this.permissionsService},
    );
  }

  /** Export Excel qua BE (lấy trọn mọi tư vấn viên, không chỉ trang đang xem). */
  exportSummary(event: Event) {
    if (!this.permissionsService.getPermission(PERMISSION_SELECT.permissionManageCrmReport)) {
      this.noti.warnMsg(this.translate.instant('common.message.no_permission'));
      return;
    }
    this.confirmationService.confirm({
      target: event.target ?? undefined,
      message: this.translate.instant('common.message.confirm_export'),
      icon: 'pi pi-exclamation-triangle',
      acceptButtonProps: {severity: 'danger', size: 'small'},
      accept: () => {
        this.isSubmitting.set(true);
        this.loadingService.showLoading('exportCrmRevenueTelesaleSummary');
        this.modelService
          .exportRevenueTelesaleSummary({params: this.exportParams()})
          .pipe(
            finalize(() => {
              this.isSubmitting.set(false);
              this.loadingService.hideLoading('exportCrmRevenueTelesaleSummary');
            }),
          )
          .subscribe({
            next: (res: any) => {
              this.noti.successMsg(this.translate.instant('common.message.export_success'));
              this.downloadExport(res);
            },
            error: () => this.noti.errorMsg(this.translate.instant('common.message.export_failed')),
          });
      },
    });
  }
}
