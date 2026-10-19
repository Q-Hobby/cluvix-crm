import { ChangeDetectionStrategy, Component, computed, inject, input, output, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { HttpContext } from '@angular/common/http';
import { finalize } from 'rxjs';
import { SKIP_GLOBAL_403, SKIP_GLOBAL_404 } from '@/core/interceptors/response-handler.interceptor';
import { TagSeverity } from 'primeng/tag';
import { TextareaModule } from 'primeng/textarea';

import { AppointmentModel } from '@/core/models/crm/appointment.model';
import { AppointmentService } from '@/core/services/crm/appointment.service';
import {
  APPOINTMENT_IS_ACCEPTED_SELECT,
  APPOINTMENT_STATUS_MAP,
  APPOINTMENT_STATUS_SELECT,
  PERMISSION_SELECT,
  appointmentStatusInfo,
} from '@/define';
import { ServicesHelperService } from '@/indexeddb/config/components/services/services-helper.service';
import { UserHelperService } from '@/indexeddb/config/components/user/user-helper.service';
import { SLIDE_OVER_IMPORTS } from '@/share/components/slide-over';
import { BaseDetailPanel } from '@/share/base/base-detail-panel';
import { WhitespaceControlDirective } from '@/share/directive/whitespace-control.directive';
import { DetailShareImports } from '@/share/import/detail-share-imports';
import { LocaleFormatService } from '@/share/services/locale-format.service';

/**
 * Luồng trạng thái chính của 1 lịch hẹn — Hủy (4) nằm ngoài luồng, xử lý riêng.
 * Icon dùng khi bước CHƯA hoàn thành; bước đã qua hiển thị dấu tích.
 */
const STATUS_FLOW: { value: number; icon: string }[] = [
  { value: APPOINTMENT_STATUS_SELECT.APPOINTMENT_STATUS_NOT_CHECKIN, icon: 'pi pi-calendar-clock' },
  { value: APPOINTMENT_STATUS_SELECT.APPOINTMENT_STATUS_CHECKED_IN, icon: 'pi pi-sign-in' },
  { value: APPOINTMENT_STATUS_SELECT.APPOINTMENT_STATUS_ADVISE, icon: 'pi pi-comments' },
  { value: APPOINTMENT_STATUS_SELECT.APPOINTMENT_STATUS_TREATMENT, icon: 'pi pi-heart' },
  { value: APPOINTMENT_STATUS_SELECT.APPOINTMENT_STATUS_CHECKED_OUT, icon: 'pi pi-sign-out' },
];

export type AppointmentStatusStep = {
  info: appointmentStatusInfo;
  icon: string;
  done: boolean;
  current: boolean;
  /**
   * Mốc thời gian thật của bước — chỉ bước check-in/check-out mới có.
   *
   * ⚠ Là `Date`, KHÔNG phải unix giây: `BaseApiService.afterLoad` đã đổi mọi field trong
   * `unixTimesTampFields` (gồm `checkin_date`/`checkout_date`) thành `Date` trước khi tới đây.
   * Khai `number` như trước là type nói dối — template nhân thêm 1000 mà `tsc` vẫn xanh, ra năm
   * 58576. Giữ đúng kiểu để lần sau máy bắt hộ.
   */
  time?: Date;
};

/**
 * Chi tiết lịch hẹn dạng SlideOver (xổ từ mép phải), khung dùng chung ở
 * `share/components/slide-over`.
 *
 * Thay cho dialog `AppointmentDetail` cũ (đã xoá): panel này khai **declarative** trong template
 * trang gọi và **tự load theo id** — trang gọi chỉ cần truyền id:
 *
 * ```html
 * <app-appointment-detail-panel
 *   [(visible)]="detail.visible"
 *   [appointmentId]="selectedId()"
 *   [seed]="selectedRow()"
 *   (edited)="openEditForm($event)" />
 * ```
 *
 * `seed` (row sẵn có ở list, đã qua `addLabel`) chỉ để vẽ ngay khung sườn, dữ liệu hiển thị vẫn
 * lấy từ `GET /appointment/{id}` — row ở list không đủ field (thiếu EMR, timeline, quan hệ).
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-appointment-detail-panel',
  standalone: true,
  templateUrl: './appointment-detail-panel.html',
  imports: [
    ...DetailShareImports,
    ...SLIDE_OVER_IMPORTS,
    TextareaModule,
    WhitespaceControlDirective,
  ],
})
export class AppointmentDetailPanel extends BaseDetailPanel<AppointmentModel> {
  readonly appointmentId = input<number | null>(null);
  /** Row sẵn có ở list (đã `addLabel`) — chỉ dùng để vẽ ngay, không thay cho API detail. */
  readonly seed = input<AppointmentModel | null>(null);

  /** Trang gọi không có luồng sửa lịch hẹn → tắt nút "Sửa" (bấm vào sẽ không có gì xảy ra). */
  readonly canEdit = input(true);

  /** Người dùng bấm "Sửa" — trang gọi tự quyết định mở form nào. */
  readonly edited = output<AppointmentModel>();

  readonly noteText = signal('');
  readonly savingNote = signal(false);

  protected readonly APPOINTMENT_STATUS_SELECT = APPOINTMENT_STATUS_SELECT;
  protected readonly APPOINTMENT_IS_ACCEPTED_SELECT = APPOINTMENT_IS_ACCEPTED_SELECT;
  protected readonly permission = PERMISSION_SELECT;

  protected readonly userHelper = inject(UserHelperService);
  private readonly appointmentService = inject(AppointmentService);
  private readonly servicesHelper = inject(ServicesHelperService);
  private readonly localeFormat = inject(LocaleFormatService);

  // Hook của BaseDetailPanel — máy móc nạp/race/lỗi/đóng nằm ở base.
  protected currentId() {
    return this.appointmentId();
  }
  protected currentSeed() {
    return this.seed();
  }
  protected fetchDetail(id: number) {
    // 404 = bản ghi đã bị xoá → panel tự báo; KHÔNG để interceptor đá cả trang sang /auth/notfound
    // (mất danh sách + bộ lọc người dùng đang mở).
    // story-14 (debt-cleanup): 403 = mất quyền trên đúng lịch hẹn này (deep-link từ chuông tới lịch
    // của chi nhánh khác) — panel tự báo qua `detailLoadErrorKey`, KHÔNG để interceptor đá cả trang
    // sang `/auth/accessdenied` (mất danh sách + bộ lọc đang mở).
    return this.appointmentService.get(id, {
      context: new HttpContext().set(SKIP_GLOBAL_404, true).set(SKIP_GLOBAL_403, true),
    });
  }
  protected override ensureCache() {
    return this.appointmentService.ensureCacheLoaded();
  }
  protected override decorate(detail: AppointmentModel): AppointmentModel {
    // KHÔNG gộp seed: `GET /crm/appointment/:id` trả đủ field màn này cần, gộp thêm row list chỉ
    // rước lại giá trị cũ hơn.
    return this.appointmentService.addLabel(detail) as AppointmentModel;
  }
  protected override onLoadStart(): void {
    this.noteText.set('');
  }

  /** Trạng thái hiện tại (map từ `status`) — `statusInfo` của seed có thể chưa có. */
  readonly statusInfo = computed<appointmentStatusInfo | undefined>(
    () => APPOINTMENT_STATUS_MAP[this.model().status],
  );

  /**
   * Severity cho tag "xác nhận lịch" — `CONFIRM_APPOINTMENT_OPTION` còn dùng chuỗi 'warning' của
   * PrimeNG đời cũ, v22 chỉ nhận 'warn' (và `TagSeverity` là union nên không truyền string trần được).
   */
  readonly confirmSeverity = computed<TagSeverity>(() => {
    const severity = this.model().confirmAppointmentInfo?.severity;
    if (severity === 'warning' || severity === 'warn') return 'warn';
    if (severity === 'success' || severity === 'danger' || severity === 'info') return severity;
    return 'secondary';
  });

  readonly isCancelled = computed(
    () => this.model().status === APPOINTMENT_STATUS_SELECT.APPOINTMENT_STATUS_CANCEL,
  );

  /**
   * Các bước của thanh trạng thái, kèm cờ đã qua / đang ở và mốc giờ thật (nếu có).
   * Giờ check-in/check-out hiển thị ngay dưới bước tương ứng nên KHÔNG lặp lại thành dòng riêng.
   */
  readonly statusSteps = computed<AppointmentStatusStep[]>(() => {
    const model = this.model();
    const current = STATUS_FLOW.findIndex((step) => step.value === Number(model.status));
    return STATUS_FLOW.map((step, index) => ({
      info: APPOINTMENT_STATUS_MAP[step.value]!,
      icon: step.icon,
      done: current > -1 && index < current,
      current: index === current,
      // `afterLoad` đã đổi checkin_date/checkout_date sang `Date`, nhưng `AppointmentModel` vẫn
      // khai `number` (nợ kiểu app-wide, xem chú thích trong model). Ép kiểu ĐÚNG MỘT CHỖ ở đây
      // để phần còn lại — kể cả template — làm việc với kiểu thật.
      time:
        step.value === APPOINTMENT_STATUS_SELECT.APPOINTMENT_STATUS_CHECKED_IN
          ? (model.checkin_date as unknown as Date | undefined)
          : step.value === APPOINTMENT_STATUS_SELECT.APPOINTMENT_STATUS_CHECKED_OUT
            ? (model.checkout_date as unknown as Date | undefined)
            : undefined,
    }));
  });

  /** Thời lượng dự kiến — `schedule_time` lưu bằng GIÂY. */
  readonly durationMinutes = computed(() => {
    const seconds = this.model().schedule_time ?? 0;
    return seconds > 0 ? Math.round(seconds / 60) : 0;
  });

  readonly customerAge = computed(() => {
    const birthday = this.model().customer?.birthday;
    if (!birthday) return 0;
    // birthday là timestamp unix (giây) sau afterLoad; phòng trường hợp BE trả chuỗi ngày.
    const time = typeof birthday === 'number' ? birthday * 1000 : new Date(birthday).getTime();
    if (!time || Number.isNaN(time)) return 0;
    const age = new Date().getFullYear() - new Date(time).getFullYear();
    return age > 0 && age < 130 ? age : 0;
  });

  readonly timeline = computed(() => this.model().noteTimeline ?? []);

  /**
   * Tên dịch vụ điều trị. BE trả `orderServiceUsed` gọn (id/code/service_id) — tên dịch vụ tra từ
   * cache IndexedDB theo `service_id`, không bắt BE JOIN thêm bảng service chỉ để lấy 1 chuỗi.
   */
  readonly treatmentServiceLabel = computed(() => {
    const used = this.model().orderServiceUsed;
    if (!used) return '';
    return this.servicesHelper.getServiceInfo(used.service_id!)?.name || used.code || '';
  });

  /**
   * Nội dung nút "Sao chép". Là `computed`, KHÔNG gọi method trong binding: hàm này tạo `Date`, gọi
   * `translate.instant` 7 lần và dựng 1 element DOM để bóc thẻ HTML — chạy lại mỗi chu kỳ change
   * detection (mỗi ký tự gõ ở ô ghi chú) là lãng phí thật.
   */
  readonly copyText = computed(() => this.buildCopyText(this.model()));


  /** Nạp lại chi tiết đang mở — trang gọi dùng sau khi sửa lịch hẹn ở form khác. */
  reload(): void {
    const id = this.model().id ?? this.appointmentId();
    if (id) this.loadDetail(Number(id));
  }




  protected submitNote(): void {
    const note = this.noteText().trim();
    const id = this.model().id;
    if (!note || !id) return;

    this.savingNote.set(true);
    this.appointmentService
      .addNote(id, note)
      .pipe(
        takeUntilDestroyed(this.destroyRef),
        finalize(() => this.savingNote.set(false)),
      )
      .subscribe((res: any) => {
        const data = res?.data ?? res;
        this.model.update((m) => ({ ...m, noteTimeline: data?.noteTimeline ?? m.noteTimeline }));
        this.noteText.set('');
      });
  }

  /** Dựng nội dung sao chép — giữ đúng bố cục text như bản dialog cũ. */
  private buildCopyText(model: AppointmentModel): string {
    const date = new Date(model.appointment_date);
    const locale = this.localeFormat.currentLocale();
    const t = this.translate;

    return [
      `${t.instant('common.label.appointment_code')}: ${model.code ?? ''}`,
      `${t.instant('common.label.appointment_time')}: ${date.toLocaleTimeString(locale, { hour: '2-digit', minute: '2-digit' })}`,
      `${t.instant('common.label.appointment_date')}: ${date.toLocaleDateString(locale)}`,
      `${t.instant('common.label.full_name')}: ${model.customer?.fullname || model.customer?.nickname || ''}`,
      `${t.instant('common.label.phone')}: ${model.customer?.phone ?? ''}`,
      `${t.instant('common.label.note')}: ${this.stripHtmlTags(model.content ?? '')}`,
      `${t.instant('common.label.branch')}: ${model.branchLabel ?? ''}`,
    ].join('\n');
  }

  /**
   * Icon + màu chữ cho từng loại sự kiện timeline (suy từ nhãn hành động BE trả về).
   * Chỉ tô màu ICON, không tô cả khối: khung ô theo mẫu stepper dọc là trung tính, mốc mới nhất mới
   * được nhấn bằng màu primary — tô màu cả khối cho mọi dòng thì không còn nhấn được cái nào.
   */
  protected getEventStyle(action: string): { icon: string; color: string } {
    const a = action?.toLowerCase() ?? '';
    if (a.includes('ghi chú') || a.includes('note')) {
      return { icon: 'pi pi-pencil', color: 'text-teal-600' };
    }
    if (a.includes('tạo') || a.includes('create')) {
      return { icon: 'pi pi-plus-circle', color: 'text-blue-600' };
    }
    if (a.includes('huỷ') || a.includes('hủy') || a.includes('cancel')) {
      return { icon: 'pi pi-times-circle', color: 'text-red-500' };
    }
    if (a.includes('không đến') || a.includes('vắng')) {
      return { icon: 'pi pi-ban', color: 'text-orange-500' };
    }
    if (a.includes('đến') || a.includes('xác nhận') || a.includes('confirm')) {
      return { icon: 'pi pi-check-circle', color: 'text-green-600' };
    }
    if (a.includes('cập nhật') || a.includes('update')) {
      return { icon: 'pi pi-refresh', color: 'text-yellow-600' };
    }
    return { icon: 'pi pi-clock', color: 'text-primary' };
  }

  /**
   * Lấy text thuần từ HTML do người dùng nhập (content lịch hẹn). KHÔNG gán `innerHTML` lên element
   * của document đang chạy: element tách rời vẫn tải ảnh và bắn `onerror` ⇒ stored XSS (S-5).
   * Document của DOMParser là inert — không chạy script, không tải tài nguyên, không bắn event.
   */
  private stripHtmlTags(html: string): string {
    return new DOMParser().parseFromString(html, 'text/html').body.textContent ?? '';
  }
}
