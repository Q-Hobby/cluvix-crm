import {ChangeDetectionStrategy, Component, effect, inject, OnInit, signal, ViewChild } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Table } from 'primeng/table';
import { Observable, switchMap } from 'rxjs';

import { LocationWardForm } from '@/modules/config/config-location/location-ward/form/location-ward-form';
import { LocationWardModel } from '@/core/models/config/location/location-ward.model';
import { LocationWardService } from '@/core/services/config/location/location-ward.service';
import { BaseListV2 } from '@/share/base/base-list-v2';
import { FilterFieldDef } from '@/share/base/list-filter.model';
import { PERMISSION_SELECT, WARD_TYPE_OPTION } from '@/define';
import { ListShareImports } from '@/share/import/list-share-imports';
import { ListApiResponse } from '@/share/models/list-api-response';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-location-ward-list',
  templateUrl: './location-ward-list.html',
  styles: [],
  imports: [FillHeightDirective, ...ListShareImports],
  standalone: true,
})
export class LocationWardList extends BaseListV2<LocationWardModel> implements OnInit {
  modelService = inject(LocationWardService);
  @ViewChild('dt') dt!: Table<LocationWardModel>;
  displayCols: any = [
    { label: 'ID', labelKey: 'common.label.id', value: 'id' },
    { label: 'Mã', labelKey: 'common.label.code', value: 'codeWard' },
    { label: 'Phường / Xã', labelKey: 'config.location.location_ward.name', value: 'name' },
    { label: 'Loại', labelKey: 'common.label.type', value: 'locationWardType' },
    { label: 'Tỉnh / Thành', labelKey: 'common.label.province', value: 'province_id' },
    { label: 'Trạng thái', labelKey: 'common.label.status', value: 'status' },
    { label: 'Vị trí', labelKey: 'common.label.position', value: 'SortOrder' },
  ];
  override selectedColumns = signal([
    'id',
    'codeWard',
    'name',
    'locationWardType',
    'province_id',
    'status',
    'SortOrder',
  ]);
  protected readonly WARD_TYPE_OPTION = WARD_TYPE_OPTION;
  override createPermissionString = PERMISSION_SELECT.permissionManageConfig;
  override updatePermissionString = PERMISSION_SELECT.permissionManageConfig;
  protected override URL_STATE_PREFIX = 'lw';

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
   * Khai báo filter — CHỈ `_keyword`. `province_id` của V1 đã bị GỠ: BE `ListLocationWard`
   * không đọc field này ở bất kỳ dạng nào (`ApplyFilter`/`GetFilterValues`/`ApplyKeywordFilter`/
   * `c.Query("province_id"`/`c.Query("filter[province_id]"` đều 0 hit trong đúng hàm list — hit
   * duy nhất của `province_id` nằm ở hàm KHÁC `GetLocationWardByProvince`, không phải list).
   * Control này vốn đã chết từ V1 (đổi URL, không đổi kết quả) — xem `deferred-work.md`.
   */
  protected filterFields(): FilterFieldDef[] {
    return [{ name: '_keyword', kind: 'text' }];
  }

  protected override fetchList(): Observable<ListApiResponse<LocationWardModel>> {
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
  override createRecord(): void {
    if (
      this.createPermissionString &&
      !this.permissionsService.getPermission(this.createPermissionString)
    ) {
      this.noti.warnMsg(this.translate.instant('common.message.no_permission'));
      return;
    }
    const header = this.translate.instant('common.action.create');
    const dialogRef = this.dialogWrapperService.open(LocationWardForm, {
      width: '700px',
      header,
    });
    dialogRef.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((result: LocationWardModel) => {
      if (result) {
        this.getList();
      }
    });
  }

  /**
   * Mở dialog cập nhật bản ghi
   */
  override updateRecord(model: LocationWardModel) {
    if (
      this.updatePermissionString &&
      !this.permissionsService.getPermission(this.updatePermissionString)
    ) {
      this.noti.warnMsg(this.translate.instant('common.message.no_permission'));
      return;
    }
    const header = this.translate.instant('common.action.update');
    const dialogRef = this.dialogWrapperService.open(LocationWardForm, {
      width: '700px',
      header,
      data: { model },
    });
    dialogRef.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((result: LocationWardModel) => {
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
