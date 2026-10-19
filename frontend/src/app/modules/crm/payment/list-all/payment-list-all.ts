import { ChangeDetectionStrategy, Component, computed, effect, inject, OnInit, signal, ViewChild } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { Table } from 'primeng/table';
import { finalize, Observable, switchMap } from 'rxjs';

import { PaymentModel } from '@/core/models/crm/payment.model';
import { PaymentService } from '@/core/services/crm/payment.service';
import { DEPARTMENT_SELECT, PAYMENT_FOR_OPTION, PERMISSION_SELECT, TYPE_DEPOSIT_OPTION } from '@/define';
import { GroupCountryMultiselectFromControl } from '@/indexeddb/config/components/group-country';
import { GroupLocalMultiselectFromControl } from '@/indexeddb/config/components/group-local';
import { PaymentMethodMultiselectFormControl } from '@/indexeddb/config/components/payment-method';
import { PaymentTypeMultiselectFormControl } from '@/indexeddb/config/components/payment-type';
import { CustomerTimeline } from '@/modules/crm/customer/contact-list/customer-timeline/customer-timeline';
import { PaymentDetailPanel } from '@/modules/crm/payment/detail-panel/payment-detail-panel';
import { ExportMode, ExportOptionsDialog } from '@/modules/utilities/export-options-dialog/export-options-dialog';
import { BaseListV2 } from '@/share/base/base-list-v2';
import { detailPanelHost } from '@/share/base/detail-panel-host';
import { FilterFieldDef } from '@/share/base/list-filter.model';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';
import { ListShareImports } from '@/share/import/list-share-imports';
import { ListApiResponse } from '@/share/models/list-api-response';

import { PAYMENT_LIST_KINDS, PaymentFilterName, PaymentListKind, PaymentListKindConfig } from './payment-list-kinds';

/**
 * 6 màn danh sách phiếu (thanh toán, thu/chi, đặt cọc, ký gửi, hoàn cọc, hoàn dịch vụ) — port từ ERP
 * `accountant/payment/list-*`, gộp thành MỘT component tham số hoá theo `route.data.kind`
 * (cấu hình cột/bộ lọc/tổng riêng từng màn ở `payment-list-kinds.ts`).
 *
 * Mỗi route là 1 config riêng ⇒ Angular tạo instance MỚI khi chuyển giữa các màn (không tái dùng
 * component), nên cấu hình đọc 1 lần trong constructor là đủ. `URL_STATE_PREFIX`/khoá localStorage
 * khác nhau từng màn ⇒ filter trên URL và cột đã chọn không lẫn giữa các màn.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-payment-list-all',
  templateUrl: './payment-list-all.html',
  standalone: true,
  imports: [
    FillHeightDirective,
    ...ListShareImports,
    PaymentDetailPanel,
    PaymentMethodMultiselectFormControl,
    PaymentTypeMultiselectFormControl,
    GroupLocalMultiselectFromControl,
    GroupCountryMultiselectFromControl,
  ],
})
export class PaymentListAll extends BaseListV2<PaymentModel> implements OnInit {
  modelService = inject(PaymentService);
  @ViewChild('dt') dt!: Table;

  // Đặt tên khác `activatedRoute` — BaseListV2 đã khai field private cùng tên (TS2415).
  private readonly pageRoute = inject(ActivatedRoute);
  readonly kind: PaymentListKind = (this.pageRoute.snapshot.data['kind'] as PaymentListKind) ?? 'payment';
  readonly cfg: PaymentListKindConfig = PAYMENT_LIST_KINDS[this.kind] ?? PAYMENT_LIST_KINDS.payment;

  /** SlideOver chi tiết phiếu — dòng đã qua `addLabels` nên dùng thẳng làm seed. */
  protected readonly detail = detailPanelHost<PaymentModel>();

  /** Cho AppColumnToggle + ExportOptionsDialog (đếm tổng cột). */
  displayCols = this.cfg.cols;
  /** Cột đang hiển thị, đúng thứ tự khai báo. */
  readonly visibleCols = computed(() => {
    const sel = new Set(this.selectedColumns());
    return this.cfg.cols.filter((c) => sel.has(c.value));
  });

  readonly amountTotal = signal(0);
  readonly depositAfterTotal = signal(0);
  private fetchSeq = 0;

  private readonly filterSet = new Set<PaymentFilterName>(this.cfg.filters);
  hasFilter = (name: PaymentFilterName) => this.filterSet.has(name);

  readonly paymentForOptions = PAYMENT_FOR_OPTION.map((o) => ({ value: o.value, label: this.translate.instant(o.labelKey) }));
  readonly typeDepositOptions = TYPE_DEPOSIT_OPTION.map((o) => ({ value: o.value, label: this.translate.instant(o.labelKey) }));
  /** Lọc tư vấn viên: chỉ nhân sự phòng Telesale. */
  readonly telesaleDepartment = DEPARTMENT_SELECT.telesale;

  constructor() {
    super();
    this.URL_STATE_PREFIX = this.cfg.urlPrefix;
    this.LS_SELECTED_COLUMNS_KEY = this.cfg.lsColumnsKey;
    this.selectedColumns.set([...this.cfg.selected]);
    // Sort mặc định ở constructor (TRƯỚC initListV2()) để base chốt làm initialSort.
    if (this.cfg.defaultSort) this.sort.set(this.cfg.defaultSort);
    effect(() => this.padCols.set(this.visibleCols().length + 2)); // + cột # và cột thao tác
  }

  ngOnInit() {
    this.initListV2();
  }

  /** Lọc bỏ cột đã lưu localStorage không còn trong `cfg.cols` (vd `marketing` đã gỡ 2026-10-18). */
  override loadSelectedColumns() {
    super.loadSelectedColumns();
    const valid = new Set<string>(this.cfg.cols.map((c) => c.value));
    this.selectedColumns.update((cols) => cols.filter((c) => valid.has(c)));
  }

  /**
   * `_keyword`, `_listRevenues`, `_groupLocal`, `_groupCountry`: BE đọc qua ApplyKeywordFilter /
   * GetFilterValues / ApplyFilter (crm/payment/handler/list.go) — nhận đúng param buildFilter sinh ra.
   */
  protected filterFields(): FilterFieldDef[] {
    const defs: FilterFieldDef[] = [{ name: '_keyword', kind: 'text' }];
    for (const name of this.cfg.filters) {
      defs.push(name === 'created_time' ? { name, kind: 'dateRange' } : { name, kind: 'in' });
    }
    defs.push({
      name: this.cfg.dateField,
      kind: 'dateRange',
      default: () => [this.timezoneService.monthStart(), this.timezoneService.todayEnd()],
    });
    return defs;
  }

  /**
   * Response `{object: {items, _meta}, <field tổng>}` → bóc ở đây. Field tổng là side-effect ngoài
   * `items` ⇒ có sequence guard: response cũ về muộn không được ghi đè tổng của lần lọc mới.
   */
  protected override fetchList(): Observable<ListApiResponse<PaymentModel>> {
    const seq = ++this.fetchSeq;
    return this.modelService.listPaymentKind(this.cfg.subPath, { params: this.searchParams() }).pipe(
      switchMap(async (response: any) => {
        const items = await this.modelService.addLabels(response?.['object']?.items ?? []);
        if (seq === this.fetchSeq) {
          this.amountTotal.set(response?.[this.cfg.sumField] ?? 0);
          if (this.cfg.depositAfterField) this.depositAfterTotal.set(response?.[this.cfg.depositAfterField] ?? 0);
        }
        return { items, _meta: response?.['object']?._meta } as ListApiResponse<PaymentModel>;
      }),
    );
  }

  openPaymentDetail(model: PaymentModel) {
    this.detail.open(model);
  }

  timelineCustomer(event: Event, id: number, fullname: string) {
    this.dialogRef = this.dialogWrapperService.open(CustomerTimeline, {
      width: '960px',
      data: { customerId: id },
      header: fullname,
    });
  }

  onQuickSelectDate({ start, end }: { start: Date; end: Date }) {
    this.rfFilterModel.patchValue({ [this.cfg.dateField]: [start, end] });
  }

  onQuickSelectCreatedTime({ start, end }: { start: Date; end: Date }) {
    this.rfFilterModel.patchValue({ created_time: [start, end] });
  }

  clearDate() {
    this.setFilterValue(this.cfg.dateField, null);
  }

  clearCreatedTime() {
    this.setFilterValue('created_time', null);
  }

  /** Export Excel — chọn "cột đang hiện" hoặc "tất cả cột" trước. */
  exportExcel() {
    if (!this.permissionsService.getPermission(PERMISSION_SELECT.permissionExtPayment)) {
      this.noti.warnMsg(this.translate.instant('common.message.no_permission'));
      return;
    }
    const ref = this.dialogWrapperService.open(ExportOptionsDialog, {
      header: this.translate.instant('utilities.export_options_dialog.header'),
      width: '520px',
      data: {
        selectedCount: this.selectedColumns().length,
        totalCount: this.displayCols.length,
      },
    });
    ref.onClose.subscribe((mode: ExportMode | null) => {
      if (mode) this.doExport(mode);
    });
  }

  private doExport(mode: ExportMode) {
    const loadingKey = 'exportPaymentList';
    this.isSubmitting.set(true);
    this.loadingService.showLoading(loadingKey);
    const params: any = { ...this.exportParams() };
    if (mode === 'selected') {
      params['columns'] = this.selectedColumns().join(',');
    }
    this.modelService
      .exportPaymentKind(this.cfg.subPath, { params })
      .pipe(
        finalize(() => {
          this.isSubmitting.set(false);
          this.loadingService.hideLoading(loadingKey);
        }),
      )
      .subscribe({
        next: (res: any) => {
          this.noti.successMsg(this.translate.instant('common.message.export_success'));
          this.downloadExport(res);
        },
      });
  }
}
