import {ChangeDetectionStrategy, Component, effect, inject, OnInit, signal, ViewChild } from '@angular/core';
import { Table } from 'primeng/table';

import { AppointmentNotBookedReasonsForm } from '@/modules/config/config-appointment/appointment-not-booked-reasons/form/appointment-not-booked-reasons-form';
import { AppointmentNotBookedReasonsModel } from '@/core/models/config/appointment/appointment-not-booked-reasons.model';
import { AppointmentNotBookedReasonsService } from '@/core/services/config/appointment/appointment-not-booked-reasons.service';
import { BaseListV2 } from '@/share/base/base-list-v2';
import { FilterFieldDef } from '@/share/base/list-filter.model';
import { PERMISSION_SELECT } from '@/define';
import { ListShareImports } from '@/share/import/list-share-imports';
import { Observable, switchMap } from 'rxjs';
import { ListApiResponse } from '@/share/models/list-api-response';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-appointment-not-booked-reasons-list',
  templateUrl: './appointment-not-booked-reasons-list.html',
  styles: [],
  imports: [FillHeightDirective, ...ListShareImports],
  standalone: true,
})
export class AppointmentNotBookedReasonsList
  extends BaseListV2<AppointmentNotBookedReasonsModel>
  implements OnInit
{
  modelService = inject(AppointmentNotBookedReasonsService);
  @ViewChild('dt') dt!: Table<AppointmentNotBookedReasonsModel>;
  protected override URL_STATE_PREFIX = 'anbr';
  displayCols: any = [
    { label: 'ID', labelKey: 'common.label.id', value: 'id' },
    { label: 'Lý do', labelKey: 'common.label.not_booked_reason', value: 'name' },
    { label: 'Nhóm cha', labelKey: 'common.label.parent', value: 'parent_id' },
    { label: 'Mô tả', labelKey: 'common.label.description', value: 'desc' },
    { label: 'Trạng thái', labelKey: 'common.label.status', value: 'status' },
    { label: 'Vị trí', labelKey: 'common.label.position', value: 'SortOrder' },
  ];
  override selectedColumns = signal(['id', 'name', 'parent_id', 'desc', 'status', 'SortOrder']);

  /** START CRUD */
  override formComponent = AppointmentNotBookedReasonsForm;  override updatePermissionString = PERMISSION_SELECT.permissionManageConfig;
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

  protected override fetchList(): Observable<ListApiResponse<AppointmentNotBookedReasonsModel>> {
    return this.modelService.list({ params: this.searchParams() }).pipe(
      switchMap(async (response: ListApiResponse<AppointmentNotBookedReasonsModel>) => {
        const items = response.items ?? [];
        response.items = await this.modelService.addLabels(items);
        return response;
      }),
    );
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
