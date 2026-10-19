import {ChangeDetectionStrategy, Component,
  effect,
  inject,
  Input,
  OnChanges,
  OnDestroy,
  OnInit,
  signal,
  ViewChild,
} from '@angular/core';

import { ReminderFormCustomer } from '@/modules/crm/reminder/form-customer/reminder-form-customer';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { CustomerModel } from '@/core/models/crm/customer.model';
import { ReminderModel } from '@/core/models/crm/reminder.model';
import { ReminderService } from '@/core/services/crm/reminder.service';
import { BaseListV2 } from '@/share/base/base-list-v2';
import { FilterFieldDef } from '@/share/base/list-filter.model';
import { ListShareImports } from '@/share/import/list-share-imports';
import { RelatedUserAvatarsComponent } from '@/share/components/elated-user-avatars';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';
import { Observable, switchMap } from 'rxjs';
import { Table } from 'primeng/table';
import { ListApiResponse } from '@/share/models/list-api-response';
import { REMINDER_STATUS_SELECT } from '@/define';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-reminder-mini-list',
  templateUrl: './reminder-of-customer.html',
  styles: [],
  standalone: true,
  imports: [...ListShareImports, RelatedUserAvatarsComponent, FillHeightDirective],
})
export class ReminderOfCustomer
  extends BaseListV2<ReminderModel>
  implements OnInit, OnChanges, OnDestroy
{
  modelService = inject(ReminderService);
  @ViewChild('dt') dt!: Table<ReminderModel>;
  @Input({ required: true }) customer!: CustomerModel;
  skipNgOnChangeFirstTime: boolean = true;
  displayCols: any = [
    { label: 'ID', value: 'id' },
    { label: 'Mã lời nhắc', labelKey: 'crm.reminder.code', value: 'code' },
    { label: 'Thời gian', labelKey: 'common.label.time', value: 'start_time' },
    { label: 'Nội dung', labelKey: 'common.label.content', value: 'note' },
    { label: 'Trạng thái', labelKey: 'common.label.status', value: 'statusReminder' },
    { label: 'Loại', labelKey: 'common.label.type', value: 'typeReminder' },
    { label: 'Người liên quan', labelKey: 'common.label.related_user', value: 'relatedUser' },
    { label: 'Người tạo', labelKey: 'common.label.user_created', value: 'created_by' },
    { label: 'Ngày tạo', labelKey: 'common.label.created_at', value: 'created_at' },
    { label: 'Người cập nhật', labelKey: 'common.label.user_updated', value: 'updated_by' },
    { label: 'Ngày cập nhật', labelKey: 'common.label.updated_at', value: 'updated_at' },
  ]; // dùng để hiển thị column table
  override selectedColumns = signal([
    'start_time',
    'note',
    'statusReminder',
    'typeReminder',
    'relatedUser',
  ]); // dùng để hiển thị column mặc định table
  protected override LS_SELECTED_COLUMNS_KEY = '/reminder/app-reminder-mini-list';
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

  /** List nhúng trong tab customer-detail — không có filter panel. */
  protected filterFields(): FilterFieldDef[] {
    return [];
  }

  ngOnChanges() {
    if (this.skipNgOnChangeFirstTime === true) {
      this.skipNgOnChangeFirstTime = false;
      return;
    }

    if (this.customer.id) {
      this.getList();
    }
  }

  protected override fetchList(): Observable<ListApiResponse<ReminderModel>> {
    if (!this.customer?.id) return this.emptyListResult();

    return this.modelService.reminderOfCustomer(this.customer.id!, { params: this.searchParams() }).pipe(
      switchMap(async (response: any) => {
        if (response.items) {
          const items = response.items ?? [];
          response.items = await this.modelService.addLabels(items);
        }
        return response;
      }),
    );
  }

  /**
   * Tạo mới lời nhắc
   */
  createReminderCustomer() {
    this.dialogRef = this.dialogWrapperService.open(ReminderFormCustomer, {
      header: this.translate.instant('crm.reminder.dialog_create'),
      width: '640px',
      data: {
        customer_id: this.customer.id,
      },
    });

    this.dialogRef.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((result: any) => {
      if (result) {
        this.getList();
      }
    });
  }

  /**
   * Cập nhật lời nhắc
   * @param model
   */
  updateReminderCustomer(model: ReminderModel) {
    this.dialogRef = this.dialogWrapperService.open(ReminderFormCustomer, {
      header: this.translate.instant('common.label.update_reminder'),
      width: '640px',
      data: {
        model,
      },
    });

    this.dialogRef.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((res: any) => {
      if (res) {
        const dataWithLabels = this.modelService.addLabel(res);
        this.updateRow(dataWithLabels as any);
      }
    });
  }

  /**
   * Đánh dấu hoàn thành
   * @param event
   * @param id
   * @param updatedAt
   */
  markForDone(event: Event, id: number, updatedAt: Date | number) {
    this.confirmationService.confirm({
      target: (event.target ?? undefined),
      message: this.translate.instant('crm.reminder_list.confirm_done'),
      icon: 'pi pi-exclamation-triangle',
      acceptButtonProps: {severity: 'danger', size: 'small'},
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

}
