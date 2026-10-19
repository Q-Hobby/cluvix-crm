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
import { InstallmentStatus } from '@/core/models/order-installment.model';

/**
 * Công nợ trả góp — BE GET crm/report/debt/installment: các kỳ còn mở (chưa thu / thu một phần),
 * kỳ quá hạn (due_date < hôm nay theo múi giờ thương hiệu, BE tính `is_overdue`) lên đầu.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-crm-installment-debts',
  templateUrl: './installment-debts.html',
  standalone: true,
  imports: [FillHeightDirective, ...ListShareImports],
})
export class InstallmentDebts extends BaseListV2<any> implements OnInit {
  modelService = inject(ReportDebtCrmService);
  @ViewChild('dt') dt!: Table;
  totalAmount = signal(0);
  totalPaid = signal(0);
  totalRemaining = signal(0);
  // key khớp colDef BE export.ExportInstallmentDebts
  displayCols: any = [
    { label: 'ID', value: 'id' },
    { label: 'Khách hàng', labelKey: 'common.label.customer', value: 'customer_id' },
    { label: 'Mã khách hàng', labelKey: 'common.label.customer_code', value: 'customer_number' },
    { label: 'Điện thoại', labelKey: 'common.label.phone', value: 'phone' },
    { label: 'Mã dịch vụ sử dụng', labelKey: 'common.label.treatment_service_code', value: 'service_used_code' },
    { label: 'Dịch vụ', labelKey: 'common.label.service', value: 'service_id' },
    { label: 'Kỳ', labelKey: 'crm.report.debt.label.period', value: 'period_number' },
    { label: 'Ngày đến hạn', labelKey: 'crm.report.debt.label.due_date', value: 'due_date' },
    { label: 'Trạng thái', labelKey: 'common.label.status', value: 'status' },
    { label: 'Chi nhánh', labelKey: 'common.label.branch', value: 'branch_id' },
  ];
  override selectedColumns = signal([
    'customer_id',
    'phone',
    'service_used_code',
    'service_id',
    'period_number',
    'due_date',
    'status',
    'branch_id',
  ]);
  protected override LS_SELECTED_COLUMNS_KEY = '/crm/report/debt/installment';
  protected override URL_STATE_PREFIX = 'din';

  constructor() {
    super();
    effect(() => {
      const len = this.selectedColumns()?.length ?? 0;
      this.padCols.set(len + 5); // # + 3 cột tiền cố định + cột thao tác
    });
  }

  ngOnInit() {
    this.initListV2();
  }

  /**
   * BE đọc 2 param tên riêng `_dueDateGte`/`_dueDateLte` (chuỗi `yyyy-MM-dd`) ⇒ `dateRangeSplit`
   * (base lo FormControl + URL), param thật ghép trong `fetchList()`/`exportQueryParams()`.
   */
  protected filterFields(): FilterFieldDef[] {
    return [
      { name: '_keyword', kind: 'text' },
      {
        name: 'due_date',
        kind: 'dateRangeSplit',
        default: () => [this.timezoneService.monthStart(), this.timezoneService.todayEnd()],
      },
      { name: 'branch_id', kind: 'in' },
    ];
  }

  onQuickSelect({ start, end }: { start: Date; end: Date }) {
    this.rfFilterModel.patchValue({ due_date: [start, end] });
  }

  clearDueDate() {
    this.setFilterValue('due_date', null);
  }

  timelineCustomer(event: Event, id: number, fullname: string) {
    this.dialogWrapperService.open(CustomerTimeline, {
      width: '960px',
      data: { customerId: id },
      header: fullname,
    });
  }

  private formatDateParam(d: Date): string {
    const y = d.getFullYear();
    const m = String(d.getMonth() + 1).padStart(2, '0');
    const day = String(d.getDate()).padStart(2, '0');
    return `${y}-${m}-${day}`;
  }

  /** Ghép `_dueDateGte`/`_dueDateLte` từ control `due_date` vào bộ param gốc. */
  private withDueDate(base: any): any {
    const dueDate: Date[] | null = this.rfFilterModel.getRawValue()['due_date'];
    const params: any = { ...base };
    if (dueDate?.[0] && dueDate?.[1]) {
      params['_dueDateGte'] = this.formatDateParam(dueDate[0]);
      params['_dueDateLte'] = this.formatDateParam(dueDate[1]);
    }
    return params;
  }

  protected override fetchList(): Observable<ListApiResponse<any>> {
    return this.modelService.listInstallmentDebts({ params: this.withDueDate(this.searchParams()) }).pipe(
      switchMap(async (response: any) => {
        const total = response?.total ?? {};
        this.totalAmount.set(total.totalAmount ?? 0);
        this.totalPaid.set(total.totalPaid ?? 0);
        this.totalRemaining.set(total.totalRemaining ?? 0);
        const items = await this.modelService.addDebtLabels(response?.items ?? []);
        return { items, _meta: response?._meta ?? null } as ListApiResponse<any>;
      }),
    );
  }

  /** Quá hạn là điều kiện NGÀY do BE tính (`is_overdue`), ưu tiên trước status. */
  isOverdue(model: any): boolean {
    return Number(model?.is_overdue) === 1;
  }

  getStatusLabel(model: any): string {
    if (this.isOverdue(model)) return this.translate.instant('crm.report.debt.status.overdue');
    if (Number(model?.status) === InstallmentStatus.PARTIAL) return this.translate.instant('crm.report.debt.status.partial');
    return this.translate.instant('crm.report.debt.status.unpaid');
  }

  getStatusSeverity(model: any): 'danger' | 'warn' | 'info' {
    if (this.isOverdue(model)) return 'danger';
    if (Number(model?.status) === InstallmentStatus.PARTIAL) return 'warn';
    return 'info';
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
    this.loadingService.showLoading('exportInstallmentDebts');
    const params = this.withDueDate(this.exportParams());
    if (mode === 'selected') {
      params['columns'] = this.selectedColumns().join(',');
    }
    this.modelService
      .exportInstallmentDebts({ params })
      .pipe(
        finalize(() => {
          this.isSubmitting.set(false);
          this.loadingService.hideLoading('exportInstallmentDebts');
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
