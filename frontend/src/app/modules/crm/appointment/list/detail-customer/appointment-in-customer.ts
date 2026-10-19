import {
  ChangeDetectionStrategy,
  Component,
  computed,
  DestroyRef,
  effect,
  EventEmitter,
  inject,
  Input,
  OnChanges,
  OnInit,
  Output,
  signal,
  SimpleChanges,
  ViewChild,
  viewChild,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { format } from 'date-fns';
import { finalize, from, map, Observable, Subject, switchMap } from 'rxjs';
import { throttleTime } from 'rxjs/operators';
import { Table } from 'primeng/table';

import {AppointmentDetailPanel} from '@/modules/crm/appointment/detail-panel/appointment-detail-panel';
import { FormAppointmentCancel } from '@/modules/crm/appointment/form/appointment-cancel/form-appointment-cancel';
import { Checkin } from '@/modules/crm/appointment/form/check-in/checkin';
import { FormAppointmentV2 } from '@/modules/crm/appointment/form/form-appointment/form-appointment-v2';
import { AppointmentModel } from '@/core/models/crm/appointment.model';
import { CustomerModel } from '@/core/models/crm/customer.model';
import { AppointmentService } from '@/core/services/crm/appointment.service';
import { BaseListV2 } from '@/share/base/base-list-v2';
import { detailPanelHost } from '@/share/base/detail-panel-host';
import { FilterFieldDef } from '@/share/base/list-filter.model';
import {
  APPOINTMENT_COLOR_STATUS_SELECT,
  APPOINTMENT_STATUS_SELECT,
  PERMISSION_SELECT,
} from '@/define';
import { WhitespaceControlDirective } from '@/share/directive/whitespace-control.directive';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';
import { ListShareImports } from '@/share/import/list-share-imports';
import { MasterDataRegistryService } from '@/indexeddb/config/service/master-data-registry.service';
import { ListApiResponse } from '@/share/models/list-api-response';
import { OrderPlanForm } from '@/modules/crm/order-plan/form/order-plan-form';
import { ConfirmCheckin } from '@/modules/crm/appointment/form/confirm-checkin/confirm-checkin';
import { OrderServiceUsedService } from '@/core/services/crm/order-service-used.service';

const isCheckinAble = (model: any, today: Date, cancelStatus: number, checkoutStatus: number): boolean => {
  const appointmentDate = typeof model.appointment_date === 'number'
    ? model.appointment_date * 1000
    : model.appointment_date?.getTime?.() ?? 0;
  const sameDay = format(today, 'yyyy-MM-dd') === format(model.appointment_date, 'yyyy-MM-dd');
  const pastNotCheckout = today.getTime() >= appointmentDate && model.status !== checkoutStatus;
  return (sameDay || pastNotCheckout) && model.status !== cancelStatus;
};

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-list-appointment-in-contact',
  templateUrl: './appointment-in-customer.html',
  styleUrls: ['./appointment-in-customer.scss'],
  standalone: true,
  imports: [...ListShareImports, AppointmentDetailPanel, WhitespaceControlDirective, FillHeightDirective],
})
export class AppointmentInCustomer extends BaseListV2<AppointmentModel> implements OnInit, OnChanges {
  modelService = inject(AppointmentService);
  orderServiceUsed = inject(OrderServiceUsedService);
  private registry = inject(MasterDataRegistryService);
  @ViewChild('dt') dt!: Table<AppointmentModel>;
  getList$ = new Subject();
  @Input({ required: true }) customer!: CustomerModel;
  @Output() saveOderTreatmentPlan: EventEmitter<boolean> = new EventEmitter<boolean>(); // load lại thông tin customer detail
  @Output() updateAppointment: EventEmitter<boolean> = new EventEmitter<boolean>(); // dùng để load lại trạng thái lịch hẹn ở màn hình list
  // column hiển thị trên table
  displayCols: any = [
    { label: 'ID', value: 'id' },
    { label: 'Mã lịch hẹn', labelKey: 'common.label.appointment_code', value: 'code' },
    { label: 'Phân loại', labelKey: 'common.label.category', value: 'bookType' },
    { label: 'Loại', labelKey: 'common.label.appointment_type', value: 'appointmentType' },
    { label: 'Thời gian hẹn', labelKey: 'common.label.appointment_time', value: 'appointmentDate' },
    { label: 'Nội dung hẹn', labelKey: 'common.label.appointment_content', value: 'content' },
    { label: 'Ghi chú nội bộ', labelKey: 'common.label.internal_note', value: 'contentPrivate' },
    { label: 'Checkin', value: 'checkin_date' },
    { label: 'Checkout', value: 'checkout_date' },
    { label: 'Lần hẹn', labelKey: 'common.label.appointment_count', value: 'appointment_number' },
    { label: 'Nhóm dịch vụ quan tâm', labelKey: 'common.label.service_group_of_interest', value: 'typeServiceInterested' },
    { label: 'Dịch vụ hẹn', labelKey: 'common.label.appointment_service', value: 'orderServiceUsed' },
    { label: 'Trạng thái', labelKey: 'common.label.status', value: 'statusName' },
    { label: 'Nhóm dịch vụ đồng ý', labelKey: 'common.label.accepted_service_group', value: 'typeServiceAccepted' },
    { label: 'Lý do', labelKey: 'common.label.reason', value: 'appointmentReason' },
    { label: 'Ghi chú lịch hẹn', labelKey: 'common.label.appointment_note', value: 'noteAppointment' },
    { label: this.translate.instant('common.label.convert'), value: 'convert_id' },
    { label: 'Tư vấn viên', labelKey: 'common.label.consultant', value: 'revenue_id' },
    { label: 'Nguồn khách hàng', labelKey: 'common.label.source', value: 'source' },
    { label: this.translate.instant('common.label.branch'), value: 'branch' },
    { label: 'Người tạo', labelKey: 'common.label.user_created', value: 'created_by' },
    { label: 'Ngày tạo', labelKey: 'common.label.created_at', value: 'created_at' },
  ];
  // select column hiển thị mặc định trên table
  override selectedColumns = signal([
    'appointmentType',
    'appointmentDate',
    'content',
    'statusName',
    'revenue_id',
  ]);
  visibleColumns = computed(() => {
    const set = new Set(this.selectedColumns());
    return Object.fromEntries(
      this.displayCols.map((c: any) => [c.value, set.has(c.value)])
    ) as Record<string, boolean>;
  });
  protected readonly APPOINTMENT_STATUS_SELECT = APPOINTMENT_STATUS_SELECT;

  /** Lọc bỏ cột đã lưu localStorage không còn trong `displayCols` (vd `marketing_id` đã gỡ 2026-10-18). */
  override loadSelectedColumns() {
    super.loadSelectedColumns();
    const valid = new Set<string>(this.displayCols.map((c: { value: string }) => c.value));
    this.selectedColumns.update((cols) => cols.filter((c) => valid.has(c)));
  }
  protected readonly APPOINTMENT_COLOR_STATUS_SELECT = APPOINTMENT_COLOR_STATUS_SELECT;

  constructor() {
    super();

    effect(() => {
      const len = this.selectedColumns()?.length ?? 0;
      this.padCols.set(len + 2);
    });
  }

  ngOnChanges(changes: SimpleChanges) {
    if (changes['customer']?.firstChange) return;
    if (this.customer?.id) {
      this.getList$.next(true);
    }
  }

  ngOnInit() {
    this.LS_SELECTED_COLUMNS_KEY = '/appointment/app-appointment-mini-customer';

    // Preload appointment types for this tab
    this.registry.loadTableOnDemand('appointmentTypes');

    this.getList$.pipe(throttleTime(500), takeUntilDestroyed(this.destroyRef)).subscribe({
      next: () => {
        this.getList();
      },
    });
    this.initListV2();
  }

  /** List nhúng trong tab customer-detail — không có filter panel. */
  protected filterFields(): FilterFieldDef[] {
    return [];
  }

  /**
   * Lấy danh sách lịch hẹn theo khách hàng
   */
  protected override fetchList(): Observable<ListApiResponse<AppointmentModel>> {
    if (!this.customer?.id) return this.emptyListResult();

    const today = this.timezoneService.now();
    const cancel = APPOINTMENT_STATUS_SELECT.APPOINTMENT_STATUS_CANCEL;
    const checkout = APPOINTMENT_STATUS_SELECT.APPOINTMENT_STATUS_CHECKED_OUT;

    return this.modelService.getAppointmentCustomer(this.customer?.id, this.searchParams()).pipe(
      switchMap((response: any) =>
        from(this.modelService.addLabels(response.items ?? [])).pipe(
          map((items: any[]) => {
            items.forEach((item: any) => {
              item._isCheckinAble = isCheckinAble(item, today, cancel, checkout);
            });
            return { ...response, items } as ListApiResponse<AppointmentModel>;
          }),
        ),
      ),
    );
  }

  async confirmAppointment(model: AppointmentModel) {
    await this.modelService.ensureCacheLoaded();
    this.dialogRef = this.dialogWrapperService.open(ConfirmCheckin, {
      header: this.translate.instant('common.label.confirm_customer_arrival'),
      width: '700px',
      data: { model },
    });
    this.dialogRef.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((res: any) => {
      if (res) {
        this.updateAppointment.emit(true);
      }
    });
  }

  /**
   * Tạo mới lịch hẹn
   */
  async createAppointmentDialog() {
    await this.modelService.ensureCacheLoaded();
    this.loadingService.showLoading('appointment-dialog');

    // Load dịch vụ khách hàng đã đặt
    this.orderServiceUsed
      .serviceUseds(this.customer?.id!)
      .pipe(finalize(() => this.loadingService.hideLoading('appointment-dialog')))
      .subscribe({
        next: (serviceTreatments) => {
          this.openAppointmentDialog(serviceTreatments || []);
        },
        error: (err) => {
          this.noti.errorMsg(this.translate.instant('common.label.cannot_load_service_list'));
          console.error('Load service error:', err);
        },
      });
  }

  private openAppointmentDialog(serviceTreatments: any[]) {
    this.dialogRef = this.dialogWrapperService.open(FormAppointmentV2, {
      width: '960px',
      header: this.translate.instant('crm.appointment.dialog.create'),
      data: {
        customer: this.customer,
        serviceTreatments,
      },
    });
    this.dialogRef.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((res: any) => {
      if (res) {
        this.updateAppointment.emit(true);
      }
    });
  }

  /**
   * Cập nhật lịch hẹn
   * @param appointment
   */
  async updateAppointmentDialog(appointment: AppointmentModel) {
    if (!this.permissionsService.getPermission(PERMISSION_SELECT.permissionUpdateAppointment)) {
      this.noti.warnMsg(this.translate.instant('common.message.no_permission'));
      return;
    }
    await this.modelService.ensureCacheLoaded();
    this.loadingService.showLoading('appointment-dialog');

    // Load dịch vụ khách hàng đã đặt
    this.orderServiceUsed
      .serviceUseds(appointment.customer_id!)
      .pipe(finalize(() => this.loadingService.hideLoading('appointment-dialog')))
      .subscribe({
        next: (serviceTreatments) => {
          this.dialogRef = this.dialogWrapperService.open(FormAppointmentV2, {
            width: '960px',
            header: this.translate.instant('common.label.update_appointment'),
            data: {
              appointment,
              serviceTreatments: serviceTreatments || [],
            },
          });
          this.dialogRef.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((data: any) => {
            if (data) {
              this.updateAppointment.emit(true);
              // SlideOver chi tiết đang mở phía sau form sửa → nạp lại cho khớp dữ liệu vừa lưu.
              if (this.detail.visible()) this.detailPanel()?.reload();
            }
          });
        },
        error: (err) => {
          this.noti.errorMsg(this.translate.instant('common.label.cannot_load_service_list'));
          console.error('Load service error:', err);
        },
      });
  }

  /**
   * Tải lại trang
   */
  override resetPage() {
    this.getList$.next(true);
  }

  /**
   * show popup hủy lịch hẹn
   * @param model
   */
  cancelAppoint(model: any) {
    this.dialogRef = this.dialogWrapperService.open(FormAppointmentCancel, {
      header: this.translate.instant('common.label.cancel_appointment'),
      width: '700px',
      data: { appointment: model },
    });
    this.dialogRef.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((data: any) => {
      if (data) {
        this.updateAppointment.emit(true);
      }
    });
  }

  /**
   * Khách hàng checkin
   * @param model
   */
  async customerCheckin(model: AppointmentModel) {
    if (this.isSubmitting()) return;
    this.isSubmitting.set(true);
    await this.modelService.ensureCacheLoaded();
    this.modelService
      .get(model.id!)
      .pipe(takeUntilDestroyed(this.destroyRef), finalize(() => this.isSubmitting.set(false)))
      .subscribe({
        next: (full: AppointmentModel) => {
          this.dialogRef = this.dialogWrapperService.open(Checkin, {
            header: this.translate.instant('common.label.customer_arrived'),
            width: '700px',
            data: { model: full },
          });
          this.dialogRef.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((data: AppointmentModel) => {
            if (data) {
              this.updateAppointment.emit(true);
            }
          });
        },
        error: (err: any) => {
          this.noti.errorMsg(err?.message || this.translate.instant('common.message.error'));
        },
      });
  }



  /**
   * Tạo mới đơn hàng hoặc dịch vụ
   */
  createOrderPlan(date: Date): void {
    if (!this.permissionsService.getPermission(PERMISSION_SELECT.permissionCreatePlanOrder)) {
      this.noti.warnMsg(this.translate.instant('common.message.no_permission'));
      return;
    }

    const data: any = {
      customer: this.customer,
    };

    const width = '960px';

    this.dialogRef = this.dialogWrapperService.open(OrderPlanForm, {
      width,
      header: this.translate.instant('common.label.add_new_order'),
      data,
    });

    this.dialogRef.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((result: any) => {
      if (result) {
        this.saveOderTreatmentPlan.emit(true);
      }
    });
  }

  /**
   * Xóa lịch hẹn
   * @param event
   * @param id
   */
  override deleteRecord(event: Event, id: number): void {
    this.isSubmitting.set(true);
    this.confirmationService.confirm({
      target: event.target as EventTarget,
      message: this.translate.instant('common.label.do_you_want_to_delete'),
      icon: 'pi pi-exclamation-triangle',
      acceptButtonProps: {severity: 'danger', size: 'small'},
      accept: () => {
        this.loadingService.showLoading('deleteRecord');
        this.modelService
          .delete(id)
          .pipe(
            finalize(() => {
              this.isSubmitting.set(false);
              this.loadingService.hideLoading('deleteRecord');
            }),
          )
          .subscribe({
            next: (res: any) => {
              this.noti.successRes(res);
              this.updateAppointment.emit(true);
            },
          });
      },
      reject: () => {
        this.isSubmitting.set(false);
      },
    });
  }

  // SlideOver chi tiết lịch hẹn
  protected readonly detail = detailPanelHost<AppointmentModel>();
  private readonly detailPanel = viewChild(AppointmentDetailPanel);

  /**
   * Mở SlideOver chi tiết lịch hẹn. Truyền cả row làm seed (row đã qua `addLabels` trong `fetchList`)
   * để panel vẽ được ngay, không phải chờ API rồi mới hiện — panel vẫn tự gọi API detail để lấy phần
   * row không có (EMR, timeline, quan hệ).
   */
  appointmentDetail(model: AppointmentModel) {
    this.detail.open(model);
  }
}
