import {ChangeDetectionStrategy, Component, effect, inject, OnInit, signal, ViewChild} from '@angular/core';
import {takeUntilDestroyed} from '@angular/core/rxjs-interop';
import {Table} from 'primeng/table';
import {finalize, Observable, switchMap} from 'rxjs';

import {CustomerTimeline} from '@/modules/crm/customer/contact-list/customer-timeline/customer-timeline';
import {BaseListV2} from '@/share/base/base-list-v2';
import {FilterFieldDef} from '@/share/base/list-filter.model';
import {PERMISSION_SELECT} from '@/define';
import {ListShareImports} from '@/share/import/list-share-imports';
import {ListApiResponse} from '@/share/models/list-api-response';
import {ReportStaffCrmService} from '@/core/services/crm/report-staff-crm.service';
import {ExportMode, ExportOptionsDialog} from '@/modules/utilities/export-options-dialog/export-options-dialog';
import {FillHeightDirective} from '@/share/directive/fill-height.directive';

/**
 * Chi tiết hoa hồng tư vấn viên (`order_commission_telesale`).
 * BE: GET /crm/report/staff/commission-telesale. Port từ ERP accountant/report/gift-telesale (bỏ thương hiệu).
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-crm-report-commission-telesale',
  templateUrl: './commission-telesale.html',
  imports: [FillHeightDirective, ...ListShareImports],
  standalone: true,
})
export class CrmCommissionTelesale extends BaseListV2<any> implements OnInit {
  modelService = inject(ReportStaffCrmService);
  @ViewChild('dt') dt!: Table;

  displayCols: any = [
    {label: 'ID', labelKey: 'common.label.id', value: 'id'},
    {label: 'Mã khách hàng', labelKey: 'common.label.customer_code', value: 'customer_code'},
    {label: 'Khách hàng', labelKey: 'common.label.customer', value: 'fullname'},
    {label: 'Số điện thoại', labelKey: 'common.label.phone', value: 'phone'},
    {label: 'Mã dịch vụ sử dụng', labelKey: 'common.label.treatment_service_code', value: 'service_used_code'},
    {label: 'Mã dịch vụ', labelKey: 'crm.report.staff.service_code', value: 'service_code'},
    {label: 'Dịch vụ', labelKey: 'common.label.service', value: 'service'},
    {label: 'Tư vấn viên', labelKey: 'common.label.consultant', value: 'telesale'},
    {label: 'Tiền thu', labelKey: 'crm.report.staff.amount', value: 'amount'},
    {label: 'Hoa hồng', labelKey: 'crm.report.staff.commission', value: 'commissionTelesale'},
    {label: 'Ngày tính hoa hồng', labelKey: 'crm.report.staff.commission_date', value: 'confirm_time'},
    {label: 'Chi nhánh', labelKey: 'common.label.branch', value: 'branch'},
  ];

  totalCommission = signal<number>(0);

  override selectedColumns = signal(['fullname', 'service', 'telesale', 'amount', 'commissionTelesale', 'confirm_time', 'branch']);

  protected override LS_SELECTED_COLUMNS_KEY = '/CrmCommissionTelesale/app-crm-report-commission-telesale/';
  protected override URL_STATE_PREFIX = 'sct';

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

  /** Cả 4 field đều được BE `crm/report/staff` đọc (spec.go listFilters). */
  protected filterFields(): FilterFieldDef[] {
    return [
      {name: '_keyword', kind: 'text'},
      {name: 'revenue_id', kind: 'in'},
      {name: 'branch_id', kind: 'in'},
      {
        name: 'confirm_time',
        kind: 'dateRange',
        default: () => [this.timezoneService.monthStart(), this.timezoneService.todayEnd()],
      },
    ];
  }

  /** Response `{items, _meta, totalCommission}` — bóc dòng tổng ở đây, gắn nhãn từ cache IndexedDB. */
  protected override fetchList(): Observable<ListApiResponse<any>> {
    return this.modelService.commissionTelesale({params: this.searchParams()}).pipe(
      switchMap(async (response: any) => {
        this.totalCommission.set(Number(response?.totalCommission) || 0);
        let items = response?.items ?? [];
        try {
          items = await this.modelService.addLabelsTelesale(items);
        } catch (e) {
          console.error('[CrmCommissionTelesale] addLabels failed:', e);
        }
        return {items, _meta: response?._meta} as ListApiResponse<any>;
      }),
    );
  }

  timelineCustomer(event: Event, id: number, fullname: string) {
    this.dialogRef = this.dialogWrapperService.open(CustomerTimeline, {
      width: '960px',
      data: {customerId: id},
      header: fullname,
    });
  }

  exportExcel() {
    if (!this.permissionsService.getPermission(PERMISSION_SELECT.permissionManageCrmReport)) {
      this.noti.warnMsg(this.translate.instant('common.message.no_permission'));
      return;
    }
    const ref = this.dialogWrapperService.open(ExportOptionsDialog, {
      header: this.translate.instant('utilities.export_options_dialog.header'),
      width: '520px',
      data: {selectedCount: this.selectedColumns().length, totalCount: this.displayCols.length},
    });
    ref.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((mode: ExportMode | null) => {
      if (!mode) return;
      this.isSubmitting.set(true);
      this.loadingService.showLoading('exportCrmCommissionTelesale');
      const params: any = {...this.exportParams()};
      if (mode === 'selected') {
        params['columns'] = this.selectedColumns().join(',');
      }
      this.modelService
        .exportCommissionTelesale({params})
        .pipe(
          finalize(() => {
            this.isSubmitting.set(false);
            this.loadingService.hideLoading('exportCrmCommissionTelesale');
          }),
        )
        .subscribe({
          next: (res: any) => {
            this.noti.successMsg(this.translate.instant('common.message.export_success'));
            this.downloadExport(res);
          },
          error: () => this.noti.errorMsg(this.translate.instant('common.message.export_failed')),
        });
    });
  }

  onQuickSelect({start, end}: {start: Date; end: Date}) {
    this.rfFilterModel.patchValue({confirm_time: [start, end]});
  }

  /** Nút "xoá ngày": bỏ hẳn filter ngày, param BE không còn gte/lte. */
  clearConfirmTime() {
    this.setFilterValue('confirm_time', null);
  }
}
