import {ChangeDetectionStrategy, Component, effect, inject, OnInit, signal, ViewChild } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Table } from 'primeng/table';
import { Observable, switchMap } from 'rxjs';

import { CustomerTimeline } from '@/modules/crm/customer/contact-list/customer-timeline/customer-timeline';
import { ReminderFormCustomer } from '@/modules/crm/reminder/form-customer/reminder-form-customer';
import { ReminderModel } from '@/core/models/crm/reminder.model';
import { ReminderService } from '@/core/services/crm/reminder.service';
import { BaseListV2 } from '@/share/base/base-list-v2';
import { FilterFieldDef } from '@/share/base/list-filter.model';
import { ListShareImports } from '@/share/import/list-share-imports';
import { RelatedUserAvatarsComponent } from '@/share/components/elated-user-avatars';
import { ListApiResponse } from '@/share/models/list-api-response';
import { REMINDER_STATUS_OPTIONS, REMINDER_STATUS_SELECT, REMINDER_TYPE_OPTIONS } from '@/define';
import { translateOptions } from '@/share/utils/translate-options.util';
import { GroupCountryMultiselectFromControl } from '@/indexeddb/config/components/group-country';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-reminder-list',
  templateUrl: './reminder-list.html',
  styles: [],
  standalone: true,
  imports: [FillHeightDirective, ...ListShareImports, RelatedUserAvatarsComponent, GroupCountryMultiselectFromControl],
})
export class ReminderList extends BaseListV2<ReminderModel> implements OnInit {
  modelService = inject(ReminderService);
  @ViewChild('dt') dt!: Table;
  //Khởi tạo lấy ra lịch hẹn có thời gian trong từ ngày đầu tiên của tháng để ngày hiện tại
  displayCols: any = [
    { label: 'ID', value: 'id' },
    { label: this.translate.instant('common.label.customer_code'), value: 'customer_code' },
    { label: this.translate.instant('common.label.customer'), value: 'fullname' },
    { label: 'Mã lời nhắc', labelKey: 'crm.reminder.code', value: 'code' },
    { label: this.translate.instant('common.label.phone'), value: 'phone' },
    { label: 'Ngày thực hiện', labelKey: 'common.label.execution_date', value: 'start_time' },
    { label: 'Trạng thái', labelKey: 'common.label.status', value: 'statusReminder' },
    { label: 'Nội dung', labelKey: 'common.label.content', value: 'note' },
    { label: 'Loại', labelKey: 'common.label.type', value: 'typeReminder' },
    { label: 'Người liên quan', labelKey: 'common.label.related_user', value: 'relatedUser' },
    { label: 'Người tạo', labelKey: 'common.label.user_created', value: 'created_by' },
    { label: 'Ngày tạo', labelKey: 'common.label.created_at', value: 'created_at' },
    { label: 'Người cập nhật', labelKey: 'common.label.user_updated', value: 'updated_by' },
    { label: 'Ngày cập nhật', labelKey: 'common.label.updated_at', value: 'updated_at' },
  ]; // dùng để hiển thị column table
  override selectedColumns = signal([
    'fullname',
    'phone',
    'start_time',
    'statusReminder',
    'note',
    'relatedUser',
  ]);
  protected override LS_SELECTED_COLUMNS_KEY = '/reminder/app-reminder-list';
  protected override URL_STATE_PREFIX = 'rm';
  protected readonly reminderTypeOptions = translateOptions(REMINDER_TYPE_OPTIONS);
  protected readonly reminderStatusOptions = translateOptions(REMINDER_STATUS_OPTIONS);
  protected readonly REMINDER_STATUS_SELECT = REMINDER_STATUS_SELECT;

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

  /**
   * Khai báo filter — base tự sinh FormGroup, đấu valueChanges → dt.filter(),
   * dựng filters khởi tạo cho p-table, lo clearFilter() và đồng bộ URL.
   */
  protected filterFields(): FilterFieldDef[] {
    return [
      { name: '_keyword', kind: 'text' },
      {
        name: 'start_time',
        kind: 'dateRange',
        // Mặc định cũ (defaultSearch): filter[start_time][gte]=monthStartUnix, [lte]=todayEndUnix.
        // buildFilterTimestamp([monthStart(), todayEnd()]) cho đúng 2 giá trị unix này (AC-R1).
        default: () => [this.timezoneService.monthStart(), this.timezoneService.todayEnd()],
      },
      { name: 'created_at', kind: 'dateRange' },
      // Chọn-một: chỉ có 2 giá trị loại-trừ nhau (lên kế hoạch / đã thực hiện), chọn cả 2 tương
      // đương không lọc. User chốt 2026-08-03 (deferred-work D1). Template bỏ [multiple], thêm
      // [showClear] — thiếu showClear thì single-select không xoá được filter.
      { name: 'status', kind: 'equals' },
      // Type có 4 giá trị không loại-trừ-nhau (gọi điện/chat/công việc/gặp mặt) — giữ multi-select.
      { name: 'type', kind: 'in' },
      { name: 'created_by', kind: 'in' },
      { name: '_relatedUser', kind: 'in' },
      { name: '_groupCountry', kind: 'in' },
    ];
  }

  /**
   * lấy danh sách lời nhắc
   * Override `fetchList` chứ KHÔNG override `searchHandler`: base cho mọi lần tải đi qua một
   * stream có `switchMap` để huỷ request cũ, và tự lo `loading`/`models`/`paginationInfo`.
   */
  protected override fetchList(): Observable<ListApiResponse<ReminderModel>> {
    return this.modelService
      .list({ params: this.searchParams() })
      .pipe(
        switchMap(async (response: any) => {
          if (response.items) {
            const items = response.items ?? [];
            try {
              response.items = await this.modelService.addLabels(items);
            } catch (e) {
              console.error('[ReminderList] addLabels failed:', e);
              response.items = items;
            }
          }
          return response;
        }),
      );
  }

  /**
   * Timeline khách hàng
   * @param event
   * @param id
   * @param fullname
   */
  timelineCustomer(event: Event, id: number, fullname: string) {
    this.dialogRef = this.dialogWrapperService.open(CustomerTimeline, {
      width: '960px',

      data: {
        customerId: id,
      },
      header: fullname,
    });
  }

  /**
   * Update lời nhắc
   * @param model
   */
  updateReminder(model: ReminderModel) {
    this.dialogRef = this.dialogWrapperService.open(ReminderFormCustomer, {
      data: { model },
      header: this.translate.instant('common.label.update_reminder'),
      width: '520px',
    });

    this.dialogRef.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((res: any) => {
      if (res) {
        const dataWithLabels = this.modelService.addLabel(res);
        this.updateRow(dataWithLabels as any);
      }
    });
  }

  onQuickSelectStartTime({ start, end }: { start: Date; end: Date }) {
    this.rfFilterModel.patchValue({ start_time: [start, end] });
  }

  onQuickSelectCreatedAt({ start, end }: { start: Date; end: Date }) {
    this.rfFilterModel.patchValue({ created_at: [start, end] });
  }

  /**
   * Đánh dấu hoàn thành
   * @param event
   * @param id
   */
  markForDone(event: Event, id: number, updatedAt: Date | number) {
    this.confirmationService.confirm({
      target: (event.target ?? undefined),
      message: this.translate.instant('crm.reminder_list.confirm_done'),
      icon: 'pi pi-exclamation-triangle',
      acceptButtonProps: {severity: 'danger', size: 'small'},
      position: 'auto', // 👉 cho phép PrimeNG tự tính toán vị trí
      accept: () => {
        this.modelService.update(id, { status: REMINDER_STATUS_SELECT.STATUS_DONE, updated_at: updatedAt }).subscribe({
          next: (res: any) => {
            if (res.data) {
              const dataWithLabels = this.modelService.addLabel(res.data);
              this.updateRow(dataWithLabels as any);
            }
            this.noti.successRes(res);
          },
        });
      },
    });
  }

  clearStartTime() {
    this.setFilterValue('start_time', null);
  }

  clearCreatedStartTime() {
    this.setFilterValue('created_at', null);
  }
}
