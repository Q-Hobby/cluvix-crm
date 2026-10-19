import {ChangeDetectionStrategy, Component, effect, inject, OnInit, signal, ViewChild } from '@angular/core';
import { Table } from 'primeng/table';

import { AppointmentIsAcceptedForm } from '@/modules/config/config-appointment/appointment-is-accepted/form/appointment-is-accepted-form';
import { AppointmentIsAcceptedModel } from '@/core/models/config/appointment/appointment-is-accepted.model';
import { AppointmentIsAcceptedService } from '@/core/services/config/appointment/appointment-is-accepted.service';
import { BaseListV2 } from '@/share/base/base-list-v2';
import { FilterFieldDef } from '@/share/base/list-filter.model';
import { PERMISSION_SELECT } from '@/define';
import { ListShareImports } from '@/share/import/list-share-imports';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-appointment-is-accepted-list',
  templateUrl: './appointment-is-accepted-list.html',
  styles: [],
  imports: [FillHeightDirective, ...ListShareImports],
  standalone: true,
})
export class AppointmentIsAcceptedList
  extends BaseListV2<AppointmentIsAcceptedModel>
  implements OnInit
{
  @ViewChild('dt') dt!: Table<AppointmentIsAcceptedModel>;
  protected override URL_STATE_PREFIX = 'aia';
  displayCols: any = [
    { label: 'ID', labelKey: 'common.label.id', value: 'id' },
    { label: 'Trạng thái tư vấn', labelKey: 'common.label.consultation_status', value: 'name' },
    { label: 'Màu sắc', labelKey: 'common.label.color', value: 'severity' },
    { label: 'Mô tả', labelKey: 'common.label.description', value: 'desc' },
    { label: 'Tính là đồng ý', labelKey: 'config.appointment_is_accepted.is_accepted', value: 'isAccepted' },
    { label: 'Trạng thái', labelKey: 'common.label.status', value: 'status' },
    { label: 'Vị trí', labelKey: 'common.label.position', value: 'SortOrder' },
  ];
  override selectedColumns = signal(['id', 'name', 'severity', 'isAccepted', 'status', 'SortOrder']);

  /** START CRUD */
  override formComponent = AppointmentIsAcceptedForm;  override updatePermissionString = PERMISSION_SELECT.permissionManageConfig;
  override createPermissionString = PERMISSION_SELECT.permissionManageConfig;  modelService = inject(AppointmentIsAcceptedService);

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
