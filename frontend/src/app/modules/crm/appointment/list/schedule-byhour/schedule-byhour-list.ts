import {
  afterNextRender,
  AfterViewInit,
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  inject,
  Injector,
  Input,
  OnChanges,
  OnInit,
  SimpleChanges,
  ViewChild,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { finalize, Subject, takeUntil } from 'rxjs';
import { Table } from 'primeng/table';
import { AppointmentModel } from '@/core/models/crm/appointment.model';
import { AppointmentService } from '@/core/services/crm/appointment.service';
import { BaseList } from '@/share/base/base-list';
import { ListShareImports } from '@/share/import/list-share-imports';
import { OverlayBadgeModule } from 'primeng/overlaybadge';
import { APPOINTMENT_STATUS_MAP, APPOINTMENT_STATUS_SELECT } from '@/define/appointment.define';
import { WhitespaceControlDirective } from '@/share/directive/whitespace-control.directive';
import { UserHelperService } from '@/indexeddb/config/components/user/user-helper.service';
import { isCheckinAble } from '@/share/utils/appointment-checkin.util';
import { FormAppointmentCancel } from '@/modules/crm/appointment/form/appointment-cancel/form-appointment-cancel';
import { Checkin } from '@/modules/crm/appointment/form/check-in/checkin';
import { FormAppointmentV2 } from '@/modules/crm/appointment/form/form-appointment/form-appointment-v2';

interface AppointmentItem {
  id: number;
  customer_id: number;
  customer_name: string;
  appointment_date: number;
  appointment_type_name: string;
  status: number;
  branch_id: number;
  branch_name: string;
  service_name: string;
  content: string;
  statusInfo?: {
    label: string;
    color: string;
    background: string;
    severity?: string;
  };
  _isCheckinAble?: boolean;
}

interface BranchGroup {
  branch_name: string;
  branch_id: number;
  appointments: AppointmentItem[];
}

const LOAD_MORE_THRESHOLD_PX = 200;
const BRANCHES_PER_PAGE = 8;

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-schedule-byhour-list',
  standalone: true,
  imports: [...ListShareImports, OverlayBadgeModule, WhitespaceControlDirective],
  templateUrl: './schedule-byhour-list.html',
})
export class ScheduleByhourList extends BaseList<any> implements OnInit, OnChanges, AfterViewInit {
  @Input() appointmentDate?: Date;
  @ViewChild(Table) table?: Table;

  modelService = inject(AppointmentService);
  protected readonly APPOINTMENT_STATUS_SELECT = APPOINTMENT_STATUS_SELECT;
  groupedData: BranchGroup[] = [];
  loadingMore = false;
  hasMore = true;
  private cdr = inject(ChangeDetectorRef);
  private userHelper = inject(UserHelperService);
  private injector = inject(Injector);
  private usersLoaded = false;
  private cancelPrevious$ = new Subject<void>();
  private requestSeq = 0;
  private branchPage = 1;
  private scrollContainer?: HTMLElement;
  private scrollListener?: () => void;

  constructor() {
    super();
    this.padCols.set(8); // Khớp số cột <th> ở #header
  }

  ngOnInit() {
    if (!this.appointmentDate) {
      queueMicrotask(() => this.searchHandler());
    }
    this.initBaseList();
  }

  ngAfterViewInit() {
    queueMicrotask(() => this.attachScrollListener());
  }

  ngOnChanges(changes: SimpleChanges) {
    if (changes['appointmentDate']?.currentValue) {
      this.searchHandler();
    }
  }

  override searchHandler() {
    this.cancelPrevious$.next();
    this.branchPage = 1;
    this.hasMore = true;
    this.groupedData = [];
    this.loadAppointments(false);
  }

  loadMore() {
    if (this.loadingMore || this.loading() || !this.hasMore) return;
    this.branchPage++;
    this.loadAppointments(true);
  }

  private attachScrollListener() {
    const container = this.table?.el?.nativeElement?.querySelector(
      '.p-datatable-table-container',
    ) as HTMLElement | null;
    if (!container) {
      // Class nội bộ PrimeNG (không phải public API) — nếu đổi tên ở version sau, infinite
      // scroll sẽ câm lặng ngừng hoạt động; log cảnh báo thay vì fail im lặng.
      console.warn('[ScheduleByhourList] Không tìm thấy .p-datatable-table-container — infinite scroll sẽ không hoạt động.');
      return;
    }
    this.scrollContainer = container;
    this.scrollListener = () => this.onTableScroll();
    container.addEventListener('scroll', this.scrollListener, { passive: true });
    this.destroyRef.onDestroy(() => {
      if (this.scrollListener) container.removeEventListener('scroll', this.scrollListener);
    });
  }

  private onTableScroll() {
    if (!this.scrollContainer) return;
    const { scrollTop, scrollHeight, clientHeight } = this.scrollContainer;
    if (scrollHeight - (scrollTop + clientHeight) < LOAD_MORE_THRESHOLD_PX) {
      this.loadMore();
    }
  }

  /**
   * Trang hiện tại có thể không đủ cao để tràn viewport (vd chi nhánh/bác sĩ ít lịch hẹn) —
   * khi đó browser không bao giờ bắn `scroll` event nên onTableScroll() không có cơ hội chạy,
   * hasMore=true nhưng data còn lại vĩnh viễn không tải được. Gọi sau mỗi lần render xong để
   * tự "lấp đầy" viewport tới khi tràn hoặc hết hasMore.
   */
  private fillViewportIfNeeded() {
    if (!this.scrollContainer || this.loadingMore || this.loading() || !this.hasMore) return;
    const { scrollHeight, clientHeight } = this.scrollContainer;
    if (scrollHeight <= clientHeight) {
      this.loadMore();
    }
  }

  private loadAppointments(append: boolean) {
    // seq chặn race: nếu ngày đổi trong lúc chờ ensureUsersLoaded() (IndexedDB async) hoặc
    // trong lúc đang loadMore(), cancelPrevious$.next() không hủy được request chưa kịp subscribe.
    const seq = ++this.requestSeq;
    if (append) {
      this.loadingMore = true;
    } else {
      this.loading.set(true);
    }
    this.loadingService.showLoading('schedule');

    const date = this.appointmentDate ?? this.timezoneService.now();
    const params = {
      appointment_date: this.toYmd(date),
      page: this.branchPage,
      per_page: BRANCHES_PER_PAGE,
    };

    const loadUsers = this.usersLoaded
      ? Promise.resolve()
      : this.userHelper.ensureUsersLoaded().then(() => {
          this.usersLoaded = true;
        });

    loadUsers.then(() => {
      if (seq !== this.requestSeq) {
        this.loadingService.hideLoading('schedule');
        return;
      }
      this.modelService
        .appointmentScheduleByHourGrouped({ params })
        .pipe(
          takeUntil(this.cancelPrevious$),
          takeUntilDestroyed(this.destroyRef),
          finalize(() => {
            this.loading.set(false);
            this.loadingMore = false;
            this.loadingService.hideLoading('schedule');
          }),
        )
        .subscribe((response: any) => {
          this.hasMore = !!response?.hasMore;
          this.processGroups(response?.groups || []);
          this.cdr.markForCheck();
          afterNextRender(() => this.fillViewportIfNeeded(), { injector: this.injector });
        });
    });
  }

  private processGroups(groups: any[]) {
    const today = this.timezoneService.now();

    const newGroups: BranchGroup[] = groups.map((g: any) => {
      const appointments: AppointmentItem[] = (g.appointments || []).map(
        (appointment: AppointmentItem) => {
          const statusNum = Number(appointment.status);
          appointment.status = statusNum;
          appointment.statusInfo = APPOINTMENT_STATUS_MAP[statusNum];
          appointment._isCheckinAble = isCheckinAble(
            appointment,
            today,
            APPOINTMENT_STATUS_SELECT.APPOINTMENT_STATUS_CANCEL,
            APPOINTMENT_STATUS_SELECT.APPOINTMENT_STATUS_CHECKED_OUT,
          );
          return appointment;
        },
      );
      return { branch_id: g.branch_id, branch_name: g.branch_name, appointments };
    });

    // BE đã sort theo branch_name và phân trang tuần tự — nối thêm, không cần sort lại.
    this.groupedData = [...this.groupedData, ...newGroups];
  }

  /**
   * Hủy lịch hẹn — fetch full model (updated_at, appointment_reason_id...) trước khi mở dialog
   * vì list schedule chỉ select field hiển thị, không đủ field cho form hủy.
   * Guard isSubmitting: tránh double-click mở nhiều dialog trong lúc chờ get(id) trả về.
   */
  cancelAppoint(appointment: AppointmentItem) {
    if (this.isSubmitting()) return;
    this.isSubmitting.set(true);
    this.modelService
      .get(appointment.id)
      .pipe(takeUntilDestroyed(this.destroyRef), finalize(() => this.isSubmitting.set(false)))
      .subscribe({
        next: (full: AppointmentModel) => {
          this.dialogRef = this.dialogWrapperService.open(FormAppointmentCancel, {
            header: this.translate.instant('common.label.cancel_appointment'),
            width: '700px',
            data: { appointment: full },
          });
          this.dialogRef.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((data: any) => {
            if (!data) return;
            this.searchHandler();
          });
        },
        error: (err: any) => {
          this.noti.errorMsg(err?.message || this.translate.instant('common.message.error'));
        },
      });
  }

  /**
   * Khách hàng checkin
   */
  async customerCheckin(appointment: AppointmentItem) {
    if (this.isSubmitting()) return;
    this.isSubmitting.set(true);
    await this.modelService.ensureCacheLoaded();
    this.modelService
      .get(appointment.id)
      .pipe(takeUntilDestroyed(this.destroyRef), finalize(() => this.isSubmitting.set(false)))
      .subscribe({
        next: (full: AppointmentModel) => {
          this.dialogRef = this.dialogWrapperService.open(Checkin, {
            header: this.translate.instant('common.label.customer_arrived'),
            width: '700px',
            data: { model: full },
          });
          this.dialogRef.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((data: any) => {
            if (!data) return;
            this.searchHandler();
          });
        },
        error: (err: any) => {
          this.noti.errorMsg(err?.message || this.translate.instant('common.message.error'));
        },
      });
  }

  /**
   * Cập nhật lịch hẹn
   */
  async updateAppointmentDialog(appointment: AppointmentItem) {
    if (!this.permissionsService.getPermission(this.permission.permissionUpdateAppointment)) {
      this.noti.warnMsg(this.translate.instant('common.message.no_permission'));
      return;
    }
    if (this.isSubmitting()) return;
    this.isSubmitting.set(true);
    await this.modelService.ensureCacheLoaded();
    this.modelService
      .get(appointment.id)
      .pipe(takeUntilDestroyed(this.destroyRef), finalize(() => this.isSubmitting.set(false)))
      .subscribe({
        next: (full: AppointmentModel) => {
          this.dialogRef = this.dialogWrapperService.open(FormAppointmentV2, {
            width: '960px',
            header: this.translate.instant('common.label.update_appointment'),
            data: { appointment: full },
          });
          this.dialogRef.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((res: any) => {
            if (res) this.searchHandler();
          });
        },
        error: (err: any) => {
          this.noti.errorMsg(err?.message || this.translate.instant('common.message.error'));
        },
      });
  }

  private toYmd(d: Date): string {
    const y = d.getFullYear();
    const m = String(d.getMonth() + 1).padStart(2, '0');
    const day = String(d.getDate()).padStart(2, '0');
    return `${y}-${m}-${day}`;
  }
}
