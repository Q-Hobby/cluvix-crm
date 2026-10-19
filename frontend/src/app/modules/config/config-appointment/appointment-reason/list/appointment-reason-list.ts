import {ChangeDetectionStrategy, Component, effect, inject, OnInit, signal, ViewChild } from '@angular/core';
import { Table } from 'primeng/table';

import { AppointmentReasonForm } from '@/modules/config/config-appointment/appointment-reason/form/appointment-reason-form';
import { AppointmentReasonModel } from '@/core/models/config/appointment/appointment-reason.model';
import { AppointmentReasonService } from '@/core/services/config/appointment/appointment-reason.service';
import { BaseListV2 } from '@/share/base/base-list-v2';
import { FilterFieldDef } from '@/share/base/list-filter.model';
import { PERMISSION_SELECT } from '@/define';
import { ListShareImports } from '@/share/import/list-share-imports';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-appointment-reason-list',
  templateUrl: './appointment-reason-list.html',
  styles: [],
  imports: [FillHeightDirective, ...ListShareImports],
  standalone: true,
})
export class AppointmentReasonList
  extends BaseListV2<AppointmentReasonModel>
  implements OnInit
{
  @ViewChild('dt') dt!: Table<AppointmentReasonModel>;
  protected override URL_STATE_PREFIX = 'ar';
  displayCols: any = [
    { label: 'ID', labelKey: 'common.label.id', value: 'id' },
    { label: 'Lý do từ chối', labelKey: 'config.appointment_reason.rejection_reason', value: 'name' },
    { label: 'Nhắc lịch', labelKey: 'common.label.reminder', value: 'time_nhaclich' },
    { label: 'Trạng thái', labelKey: 'common.label.status', value: 'status' },
    { label: 'Vị trí', labelKey: 'common.label.position', value: 'SortOrder' },
  ];
  override selectedColumns = signal(['id', 'name', 'status', 'SortOrder', 'time_nhaclich']);

  /** START CRUD */
  override formComponent = AppointmentReasonForm;  override updatePermissionString = PERMISSION_SELECT.permissionManageConfig;
  override createPermissionString = PERMISSION_SELECT.permissionManageConfig;  modelService = inject(AppointmentReasonService);

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
   * Khai báo filter — list này chỉ có ô tìm kiếm theo từ khoá.
   */
  protected filterFields(): FilterFieldDef[] {
    return [{ name: '_keyword', kind: 'text' }];
  }

  checkUpdatePermission() {
    return !this.permissionsService.getPermission(
      PERMISSION_SELECT.permissionManageConfig,
    );
  }
}
