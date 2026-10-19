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

/** Công nợ theo khách hàng — BE GET crm/report/debt/customer (customer.totalDebt != 0). */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-crm-customer-debts',
  templateUrl: './customer-debts.html',
  standalone: true,
  imports: [FillHeightDirective, ...ListShareImports],
})
export class CustomerDebts extends BaseListV2<any> implements OnInit {
  modelService = inject(ReportDebtCrmService);
  @ViewChild('dt') dt!: Table;
  totalRevenue = signal(0);
  totalPayment = signal(0);
  totalDebts = signal(0);
  // key khớp colDef BE export.ExportCustomerDebts
  displayCols: any = [
    { label: 'ID', value: 'id' },
    { label: 'Mã khách cũ', labelKey: 'common.label.old_code', value: 'customer_old_code' },
    { label: 'Mã khách hàng', labelKey: 'common.label.customer_code', value: 'customer_number' },
    { label: 'Khách hàng', labelKey: 'common.label.customer', value: 'customer_id' },
    { label: 'Điện thoại', labelKey: 'common.label.phone', value: 'phone' },
    { label: 'Doanh số', labelKey: 'common.label.sales', value: 'totalDoanhSo' },
    { label: 'Thanh toán', labelKey: 'common.label.payment', value: 'totalPayment' },
    { label: 'Công nợ', labelKey: 'common.label.debt', value: 'dept' },
    { label: 'Đặt cọc', labelKey: 'common.label.deposit', value: 'totalDeposit' },
    { label: 'Chi nhánh', labelKey: 'common.label.branch', value: 'branch_id' },
  ];
  override selectedColumns = signal([
    'customer_id',
    'phone',
    'customer_number',
    'totalDoanhSo',
    'totalPayment',
    'dept',
    'totalDeposit',
    'branch_id',
  ]);
  protected override LS_SELECTED_COLUMNS_KEY = '/crm/report/debt/customer';
  protected override URL_STATE_PREFIX = 'dcu';

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

  /** `created_day` (ngày thu thập khách) không có mặc định — mở màn là xem toàn bộ khách đang nợ. */
  protected filterFields(): FilterFieldDef[] {
    return [
      { name: '_keyword', kind: 'text' },
      { name: 'created_day', kind: 'dateRange' },
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
    this.rfFilterModel.patchValue({ created_day: [start, end] });
  }

  clearCreatedDay() {
    this.setFilterValue('created_day', null);
  }

  protected override fetchList(): Observable<ListApiResponse<any>> {
    return this.modelService.listCustomerDebts({ params: this.searchParams() }).pipe(
      switchMap(async (response: any) => {
        this.totalRevenue.set(response?.totalRevenue ?? 0);
        this.totalPayment.set(response?.totalPayment ?? 0);
        this.totalDebts.set(response?.totalDebts ?? 0);
        const items = await this.modelService.addDebtLabels(response?.items ?? []);
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
    this.loadingService.showLoading('exportCustomerDebts');
    const params = { ...this.exportParams() } as any;
    if (mode === 'selected') {
      params['columns'] = this.selectedColumns().join(',');
    }
    this.modelService
      .exportCustomerDebts({ params })
      .pipe(
        finalize(() => {
          this.isSubmitting.set(false);
          this.loadingService.hideLoading('exportCustomerDebts');
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
