import {ChangeDetectionStrategy, Component, effect, inject, OnInit, signal, ViewChild } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Table } from 'primeng/table';
import { Observable, switchMap } from 'rxjs';

import { GroupLocalForm } from '@/modules/config/config-location/group-local/form/group-local-form';
import { GroupLocalModel } from '@/core/models/config/location/group-local.model';
import { GroupLocalService } from '@/core/services/config/location/group-local.service';
import { BaseListV2 } from '@/share/base/base-list-v2';
import { FilterFieldDef } from '@/share/base/list-filter.model';
import { PERMISSION_SELECT } from '@/define';
import { ListShareImports } from '@/share/import/list-share-imports';
import { ListApiResponse } from '@/share/models/list-api-response';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-group-local-list',
  templateUrl: './group-local-list.html',
  styles: [],
  imports: [FillHeightDirective, ...ListShareImports],
  standalone: true,
})
export class GroupLocalList extends BaseListV2<GroupLocalModel> implements OnInit {
  @ViewChild('dt') dt!: Table<GroupLocalModel>;
  displayCols: any = [
    { label: 'ID', labelKey: 'common.label.id', value: 'id' },
    { label: 'Nhóm khu vực', labelKey: 'config.location.group_local.name', value: 'name' },
    { label: 'Tỉnh / Thành', labelKey: 'common.label.province', value: 'local' },
    { label: 'Trạng thái', labelKey: 'common.label.status', value: 'status' },
    { label: 'Vị trí', labelKey: 'common.label.position', value: 'SortOrder' },
  ];
  override selectedColumns = signal(['id', 'name', 'local', 'status', 'SortOrder']);
  modelService = inject(GroupLocalService);
  override createPermissionString = PERMISSION_SELECT.permissionManageConfig;
  override updatePermissionString = PERMISSION_SELECT.permissionManageConfig;
  protected override URL_STATE_PREFIX = 'gl';

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

  protected override fetchList(): Observable<ListApiResponse<GroupLocalModel>> {
    return this.modelService.list({ params: this.searchParams() }).pipe(
      switchMap(async (response: any) => {
        if (response.items) {
          const items = response.items ?? [];
          response.items = await this.modelService.addLabels(items);
        }
        return response;
      }),
    );
  }

  override createRecord(): void {
    if (
      this.createPermissionString &&
      !this.permissionsService.getPermission(this.createPermissionString)
    ) {
      this.noti.warnMsg(this.translate.instant('common.message.no_permission'));
      return;
    }
    const header = this.translate.instant('common.action.create');
    const dialogRef = this.dialogWrapperService.open(GroupLocalForm, {
      width: '700px',
      header,
    });
    dialogRef.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((result: GroupLocalModel) => {
      if (result) {
        this.getList();
      }
    });
  }

  /**
   * Mở dialog cập nhật bản ghi
   */
  override updateRecord(model: GroupLocalModel) {
    if (
      this.updatePermissionString &&
      !this.permissionsService.getPermission(this.updatePermissionString)
    ) {
      this.noti.warnMsg(this.translate.instant('common.message.no_permission'));
      return;
    }
    const header = this.translate.instant('common.action.update');
    const dialogRef = this.dialogWrapperService.open(GroupLocalForm, {
      width: '700px',
      header,
      data: { model },
    });
    dialogRef.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((result: GroupLocalModel) => {
      if (result) {
        const dataWithLabels = this.modelService.addLabel(result);
        this.updateRow(dataWithLabels as any);
      }
    });
  }

  checkUpdatePermission() {
    return !this.permissionsService.getPermission(
      PERMISSION_SELECT.permissionManageConfig,
    );
  }
}
