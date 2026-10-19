import {ChangeDetectionStrategy, Component, effect, inject, OnInit, signal, ViewChild } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Table } from 'primeng/table';
import { Observable, switchMap } from 'rxjs';

import { LocationProvinceForm } from '@/modules/config/config-location/location-province/form/location-province-form/location-province-form';
import { LocationProvinceModel } from '@/core/models/config/location/location-province.model';
import { LocationProvinceService } from '@/core/services/config/location/location-province.service';
import { BaseListV2 } from '@/share/base/base-list-v2';
import { FilterFieldDef } from '@/share/base/list-filter.model';
import { PERMISSION_SELECT, PROVINCE_TYPE_OPTION } from '@/define';
import { ListShareImports } from '@/share/import/list-share-imports';
import { ListApiResponse } from '@/share/models/list-api-response';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-location-province-list',
  templateUrl: './location-province-list.html',
  styles: [],
  imports: [FillHeightDirective, ...ListShareImports],
  standalone: true,
})
export class LocationProvinceList
  extends BaseListV2<LocationProvinceModel>
  implements OnInit
{
  modelService = inject(LocationProvinceService);
  @ViewChild('dt') dt!: Table<LocationProvinceModel>;
  displayCols: any = [
    { label: 'ID', labelKey: 'common.label.id', value: 'id' },
    { label: 'Mã', labelKey: 'common.label.code', value: 'codeProvince' },
    { label: 'Tỉnh / Thành', labelKey: 'common.label.province', value: 'name' },
    { label: 'Loại', labelKey: 'common.label.type', value: 'locationProvinceType' },
    { label: 'Quốc gia', labelKey: 'common.label.country', value: 'CountryCode' },
    { label: 'Trạng thái', labelKey: 'common.label.status', value: 'status' },
    { label: 'Vị trí', labelKey: 'common.label.position', value: 'SortOrder' },
  ];
  override selectedColumns = signal([
    'id',
    'codeProvince',
    'name',
    'locationProvinceType',
    'CountryCode',
    'status',
    'SortOrder',
  ]);

  /** START CRUD */  override updatePermissionString = PERMISSION_SELECT.permissionManageConfig;
  override createPermissionString = PERMISSION_SELECT.permissionManageConfig;  protected readonly PROVINCE_TYPE_OPTION = PROVINCE_TYPE_OPTION;
  protected override URL_STATE_PREFIX = 'lpr';

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

  protected override fetchList(): Observable<ListApiResponse<LocationProvinceModel>> {
    return this.modelService
      .list({
        params: this.searchParams(),
      })
      .pipe(
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
  override createRecord(): void {
    if (
      this.createPermissionString &&
      !this.permissionsService.getPermission(this.createPermissionString)
    ) {
      this.noti.warnMsg(this.translate.instant('common.message.no_permission'));
      return;
    }
    const header = this.translate.instant('common.action.create');
    const dialogRef = this.dialogWrapperService.open(LocationProvinceForm, {
      width: '700px',
      header,
    });
    dialogRef.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((result: LocationProvinceModel) => {
      if (result) {
        this.getList();
      }
    });
  }

  /**
   * Mở dialog cập nhật bản ghi
   */
  override updateRecord(model: LocationProvinceModel) {
    if (
      this.updatePermissionString &&
      !this.permissionsService.getPermission(this.updatePermissionString)
    ) {
      this.noti.warnMsg(this.translate.instant('common.message.no_permission'));
      return;
    }
    const header = this.translate.instant('common.action.update');
    const dialogRef = this.dialogWrapperService.open(LocationProvinceForm, {
      width: '700px',
      header,
      data: { model },
    });
    dialogRef.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((result: LocationProvinceModel) => {
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
