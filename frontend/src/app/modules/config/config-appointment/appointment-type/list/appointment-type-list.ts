import {ChangeDetectionStrategy, Component, effect, inject, OnInit, signal, ViewChild } from '@angular/core';
import { Table } from 'primeng/table';

import { AppointmentTypeForm } from '@/modules/config/config-appointment/appointment-type/form/appointment-type-form';
import { AppointmentTypeModel } from '@/core/models/config/appointment/appointment-type.model';
import { AppointmentTypeService } from '@/core/services/config/appointment/appointment-type.service';
import { BaseListV2 } from '@/share/base/base-list-v2';
import { FilterFieldDef } from '@/share/base/list-filter.model';
import { PERMISSION_SELECT } from '@/define';
import { ListShareImports } from '@/share/import/list-share-imports';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-appointment-type-list',
  templateUrl: './appointment-type-list.html',
  styles: [],
  imports: [FillHeightDirective, ...ListShareImports],
  standalone: true,
})
export class AppointmentTypeList extends BaseListV2<AppointmentTypeModel> implements OnInit {
  modelService = inject(AppointmentTypeService);
  @ViewChild('dt') dt!: Table<AppointmentTypeModel>;
  displayCols: any = [
    { label: 'ID', value: 'id' },
    { label: 'Loại lịch hẹn', labelKey: 'common.label.appointment_type', value: 'name' },
    { label: 'Loại hẹn tư vấn', labelKey: 'appointment.label.consultation_type', value: 'isTypeRevenue' },
    { label: 'Trạng thái', labelKey: 'common.label.status', value: 'status' },
    { label: 'Vị trí', labelKey: 'common.label.sort_order', value: 'SortOrder' },
  ];
  override selectedColumns = signal(['id', 'name', 'isTypeRevenue', 'status', 'SortOrder']);
  protected override LS_SELECTED_COLUMNS_KEY = '/config/appointment-type-list';
  protected override URL_STATE_PREFIX = 'at';

  /** START CRUD */
  override formComponent = AppointmentTypeForm;  override updatePermissionString = PERMISSION_SELECT.permissionManageConfig;
  override createPermissionString = PERMISSION_SELECT.permissionManageConfig;
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

  /**
   * @param id
   */
  handleTypeRevenueChange(id: number) {
    this.modelService.handleTypeRevenueChange({ id }).subscribe((res: any) => {
      this.noti.successRes(res);
    });
  }

  checkUpdatePermission() {
    return !this.permissionsService.getPermission(
      PERMISSION_SELECT.permissionManageConfig,
    );
  }

  override onCellEditComplete(e: any) {
    const id = e.data?.id;
    const field = e.field;
    const key = `${id}:${field}`;
    const prev = this.prevCellValues.get(key);
    const curr = e.data?.[field];

    if (String(prev) === String(curr)) {
      this.prevCellValues.delete(key);
      return;
    }

    const patch: any = { [field]: Number(curr), version: e.data?.version };

    this.modelService.update(id, patch).subscribe({
      next: (res: any) => {
        this.updateRow(res);
        this.noti.success(this.translate.instant('common.label.position_saved_prev_curr', { prev, curr }));
        this.prevCellValues.delete(key);
      },
      error: (err) => {
        console.error('BE error:', err);
        if (e.data) this.updateRow({ ...e.data, [field]: prev });
        this.noti.error(this.translate.instant('common.label.save_error'), err?.error?.message || err?.message || '');
        this.prevCellValues.delete(key);
      },
    });
  }
}
