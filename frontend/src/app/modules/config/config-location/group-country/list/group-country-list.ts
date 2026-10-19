import {ChangeDetectionStrategy, Component, effect, inject, OnInit, signal, ViewChild } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Table } from 'primeng/table';
import { Observable, switchMap } from 'rxjs';

import { GroupCountryForm } from '@/modules/config/config-location/group-country/form/group-country-form';
import { GroupCountryModel } from '@/core/models/config/location/group-country.model';
import { BaseListV2 } from '@/share/base/base-list-v2';
import { FilterFieldDef } from '@/share/base/list-filter.model';
import { PERMISSION_SELECT } from '@/define';
import { ListShareImports } from '@/share/import/list-share-imports';
import { GroupCountryService } from '@/core/services/config/location/group-country.service';
import { ListApiResponse } from '@/share/models/list-api-response';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-group-country-list',
  templateUrl: './group-country-list.html',
  styles: [],
  imports: [FillHeightDirective, ...ListShareImports],
  standalone: true,
})
export class GroupCountryList extends BaseListV2<GroupCountryModel> implements OnInit {
  modelService = inject(GroupCountryService);
  @ViewChild('dt') dt!: Table<GroupCountryModel>;
  displayCols: any = [
    { label: 'ID', labelKey: 'common.label.id', value: 'id' },
    { label: 'Nhóm quốc gia', labelKey: 'config.location.group_country.name', value: 'name' },
    { label: 'Quốc gia', labelKey: 'common.label.country', value: 'country' },
    { label: 'Trạng thái', labelKey: 'common.label.status', value: 'status' },
    { label: 'Vị trí', labelKey: 'common.label.position', value: 'SortOrder' },
  ];
  override selectedColumns = signal(['id', 'name', 'country', 'status', 'SortOrder']);
  override createPermissionString = PERMISSION_SELECT.permissionManageConfig;
  override updatePermissionString = PERMISSION_SELECT.permissionManageConfig;
  protected override URL_STATE_PREFIX = 'gc';

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

  protected override fetchList(): Observable<ListApiResponse<GroupCountryModel>> {
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

  /**
   * Mở dialog tạo mới bản ghi
   */
  override async createRecord() {
    if (
      this.createPermissionString &&
      !this.permissionsService.getPermission(this.createPermissionString)
    ) {
      this.noti.warnMsg(this.translate.instant('common.message.no_permission'));
      return;
    }
    const header = this.translate.instant('common.action.create');
    await this.modelService.ensureCacheLoaded();
    const dialogRef = this.dialogWrapperService.open(GroupCountryForm, {
      width: '700px',
      header,
    });
    dialogRef.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((result: GroupCountryModel) => {
      if (result) {
        this.getList();
      }
    });
  }

  /**
   * Mở dialog cập nhật bản ghi
   */
  override async updateRecord(model: GroupCountryModel) {
    if (
      this.updatePermissionString &&
      !this.permissionsService.getPermission(this.updatePermissionString)
    ) {
      this.noti.warnMsg(this.translate.instant('common.message.no_permission'));
      return;
    }
    const header = this.translate.instant('common.action.update');
    await this.modelService.ensureCacheLoaded();
    const dialogRef = this.dialogWrapperService.open(GroupCountryForm, {
      width: '700px',
      header,
      data: { model },
    });
    dialogRef.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((result: GroupCountryModel) => {
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
