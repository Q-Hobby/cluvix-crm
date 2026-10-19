import {ChangeDetectionStrategy, Component, effect, inject, OnInit, ViewChild } from '@angular/core';
import { Table } from 'primeng/table';
import { Observable, switchMap } from 'rxjs';

import { ReminderModel } from '@/core/models/crm/reminder.model';
import { userId } from '@/core/services/auth';
import { CustomerService } from '@/core/services/crm/customer.service';
import { ReminderService } from '@/core/services/crm/reminder.service';
import { BaseListV2 } from '@/share/base/base-list-v2';
import { FilterFieldDef } from '@/share/base/list-filter.model';
import { ListShareImports } from '@/share/import/list-share-imports';
import { RelatedUserAvatarsComponent } from '@/share/components/elated-user-avatars';
import { REMINDER_STATUS_SELECT } from '@/define';
import { ListApiResponse } from '@/share/models/list-api-response';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-reminder-mine-today-list',
  templateUrl: './reminder-mine-today-list.html',
  styles: [''],
  standalone: true,
  imports: [...ListShareImports, RelatedUserAvatarsComponent],
})
export class ReminderMineTodayList extends BaseListV2<ReminderModel> implements OnInit {
  modelService = inject(ReminderService);
  customerService = inject(CustomerService);
  // Widget "lời nhắc hôm nay" nhúng trong dashboard/trang khác → không được ghi state lên URL trang cha.
  @ViewChild('dt') dt!: Table<ReminderModel>;
  plus = 210;
  protected readonly REMINDER_STATUS_SELECT = REMINDER_STATUS_SELECT;

  constructor() {
    super();
    this.sort.set('-start_time');
    effect(() => {
      const len = this.selectedColumns()?.length ?? 0;
      this.padCols.set(len + 8);
    });
  }

  ngOnInit() {
    this.initListV2();
  }

  // Không có filter panel — lọc cố định (user hiện tại, hôm nay, trạng thái đã lên lịch) giữ
  // nguyên trong fetchList() qua requiredSearch, đó không phải filter người dùng.
  protected filterFields(): FilterFieldDef[] {
    return [];
  }

  /**
   * Hàm xử lý đánh dấu hoàn thành
   * @param event
   * @param id
   */
  markForDone(event: Event, id: number) {
    this.confirmationService.confirm({
      target: (event.target ?? undefined),
      message: this.translate.instant('crm.reminder_list.confirm_done'),
      icon: 'pi pi-exclamation-triangle',
      acceptButtonProps: {severity: 'danger', size: 'small'},
      accept: () => {
        const row = this.models().find((m: any) => m.id === id) as any;
        this.modelService
          .update(id, { status: REMINDER_STATUS_SELECT.STATUS_DONE, updated_at: row?.updated_at })
          .subscribe((res: any) => {
            if (res.data) {
              const dataWithLabels = this.modelService.addLabel(res.data);
              this.updateRow(dataWithLabels as any);
            }
            this.noti.successMsg(this.translate.instant('crm.reminder.noti_complete_success'));
          });
      },
    });
  }

  /**
   * Lấy danh sách lời nhắc hôm nay.
   * Override `fetchList` chứ KHÔNG override `searchHandler`: base cho mọi lần tải đi qua một
   * stream có `switchMap` để huỷ request cũ, và tự lo `loading`/`models`/`paginationInfo`.
   * Bộ lọc cố định (user hiện tại, hôm nay, trạng thái đã lên lịch) giữ NGUYÊN — không phải
   * filter người dùng nên không đưa vào `filterFields()`.
   */
  protected override fetchList(): Observable<ListApiResponse<ReminderModel>> {
    const params = {
      _relatedUser: [userId()], // Changed: use array format for JSON filtering
      'filter[status][eq]': REMINDER_STATUS_SELECT.STATUS_PLANED,
      'filter[start_time][gte]': 0,
      'filter[start_time][lte]': this.timezoneService.todayEndUnix(),
      ...this.searchParams(),
    } as any;

    return this.modelService.list({ params }).pipe(
      switchMap(async (response: any) => {
        if (response.items) {
          const items = response.items ?? [];
          response.items = await this.modelService.addLabels(items);
        }
        return response;
      }),
    );
  }
}
