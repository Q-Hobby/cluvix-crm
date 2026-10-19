import { ChangeDetectionStrategy, Component, effect, inject, OnInit, signal, ViewChild } from '@angular/core';
import { Table } from 'primeng/table';
import { finalize, Observable, switchMap } from 'rxjs';

import { CustomerTimeline } from '@/modules/crm/customer/contact-list/customer-timeline/customer-timeline';
import { BaseListV2 } from '@/share/base/base-list-v2';
import { FilterFieldDef } from '@/share/base/list-filter.model';
import { PERMISSION_SELECT } from '@/define';
import { ListShareImports } from '@/share/import/list-share-imports';
import { ListApiResponse } from '@/share/models/list-api-response';
import { ReportDebtCrmService } from '@/core/services/crm/report-debt-crm.service';
import { ExportOptionsDialog, ExportMode } from '@/modules/utilities/export-options-dialog/export-options-dialog';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';

/**
 * Công nợ theo đơn hàng — BE GET crm/report/debt/order-plan. Kỳ = `created_time` (filter[created_time][gte/lte]):
 * phải thu đầu kỳ / doanh thu trong kỳ / đã thu trong kỳ / phải thu cuối kỳ của từng đơn.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-crm-order-plan-debts',
  templateUrl: './order-plan-debts.html',
  standalone: true,
  imports: [FillHeightDirective, ...ListShareImports],
})
export class OrderPlanDebts extends BaseListV2<any> implements OnInit {
  modelService = inject(ReportDebtCrmService);
  @ViewChild('dt') dt!: Table;
  totalEndBefore = signal(0);
  totalEndIn = signal(0);
  daThuTrongKy = signal(0);
  totalEndOrder = signal(0);
  // key khớp colDef BE export.ExportOrderPlanDebts
  displayCols: any = [
    { label: 'ID', value: 'id' },
    { label: 'Mã khách hàng', labelKey: 'common.label.customer_code', value: 'customer_code' },
    { label: 'Mã đơn hàng', labelKey: 'common.label.order_code', value: 'order_plan_code' },
    { label: 'Khách hàng', labelKey: 'common.label.customer', value: 'customer_name' },
    { label: 'Phải thu đầu kỳ', labelKey: 'crm.report.debt.label.receivable_begin', value: 'P_T_D_K' },
    { label: 'Doanh thu trong kỳ', labelKey: 'crm.report.debt.label.revenue_in_period', value: 'D_T_T_K' },
    { label: 'Đã thu trong kỳ', labelKey: 'crm.report.debt.label.collected_in_period', value: 'DA_T_T_K' },
    { label: 'Phải thu cuối kỳ', labelKey: 'crm.report.debt.label.receivable_end', value: 'P_T_C_K' },
    { label: 'Ngày tạo đơn', labelKey: 'crm.report.debt.label.order_created_date', value: 'created_time' },
  ];
  override selectedColumns = signal([
    'customer_code',
    'order_plan_code',
    'customer_name',
    'P_T_D_K',
    'D_T_T_K',
    'DA_T_T_K',
    'P_T_C_K',
    'created_time',
  ]);
  protected override LS_SELECTED_COLUMNS_KEY = '/crm/report/debt/order-plan';
  protected override URL_STATE_PREFIX = 'dop';

  constructor() {
    super();
    effect(() => {
      const len = this.selectedColumns()?.length ?? 0;
      this.padCols.set(len + 2);
    });
  }

  ngOnInit() {
    this.initListV2();
  }

  /** Kỳ mặc định: đầu tháng → hết hôm nay. Xoá kỳ ⇒ BE coi đầu kỳ = 0, cuối kỳ = hiện tại. */
  protected filterFields(): FilterFieldDef[] {
    return [
      { name: '_keyword', kind: 'text' },
      {
        name: 'created_time',
        kind: 'dateRange',
        default: () => [this.timezoneService.monthStart(), this.timezoneService.todayEnd()],
      },
      { name: 'branch_id', kind: 'in' },
    ];
  }

  timelineCustomer(event: Event, id: number, fullname: string) {
    this.dialogWrapperService.open(CustomerTimeline, {
      width: '960px',
      data: { customerId: id },
      header: fullname,
    });
  }

  onQuickSelect({ start, end }: { start: Date; end: Date }) {
    this.rfFilterModel.patchValue({ created_time: [start, end] });
  }

  clearCreatedDay() {
    this.setFilterValue('created_time', null);
  }

  protected override fetchList(): Observable<ListApiResponse<any>> {
    return this.modelService.listOrderPlanDebts({ params: this.searchParams() }).pipe(
      switchMap(async (response: any) => {
        const total = response?.total ?? {};
        this.totalEndBefore.set(total.totalEndBefore ?? 0);
        this.totalEndIn.set(total.totalEndIn ?? 0);
        this.daThuTrongKy.set(total.daThuTrongKy ?? 0);
        this.totalEndOrder.set(total.totalEndOrder ?? 0);
        const items = await this.modelService.addDebtLabels(response?.items ?? [], ['created_time']);
        return { items, _meta: response?._meta ?? null } as ListApiResponse<any>;
      }),
    );
  }

  exportToExcel() {
    if (!this.permissionsService.getPermission(PERMISSION_SELECT.permissionManageCrmReport)) {
      this.noti.warnMsg(this.translate.instant('common.message.no_permission'));
      return;
    }
    const ref = this.dialogWrapperService.open(ExportOptionsDialog, {
      header: this.translate.instant('utilities.export_options_dialog.header'),
      width: '520px',
      data: { selectedCount: this.selectedColumns().length, totalCount: this.displayCols.length },
    });
    ref.onClose.subscribe((mode: ExportMode | null) => {
      if (!mode) return;
      this.doExport(mode);
    });
  }

  private doExport(mode: ExportMode) {
    this.isSubmitting.set(true);
    this.loadingService.showLoading('exportOrderPlanDebts');
    const params = { ...this.exportParams() } as any;
    if (mode === 'selected') {
      params['columns'] = this.selectedColumns().join(',');
    }
    this.modelService
      .exportOrderPlanDebts({ params })
      .pipe(
        finalize(() => {
          this.isSubmitting.set(false);
          this.loadingService.hideLoading('exportOrderPlanDebts');
        }),
      )
      .subscribe({
        next: (res: any) => {
          this.noti.successMsg(this.translate.instant('common.message.export_success'));
          this.downloadExport(res);
        },
        error: () => this.noti.errorMsg(this.translate.instant('common.message.export_failed')),
      });
  }
}
