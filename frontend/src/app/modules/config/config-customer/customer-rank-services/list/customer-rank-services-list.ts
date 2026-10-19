import {ChangeDetectionStrategy, Component, effect, inject, OnInit, signal, ViewChild } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { ListShareImports } from '@/share/import/list-share-imports';
import { BaseListV2 } from '@/share/base/base-list-v2';
import { FilterFieldDef } from '@/share/base/list-filter.model';
import { Table } from 'primeng/table';
import { Observable, switchMap } from 'rxjs';
import { ListApiResponse } from '@/share/models/list-api-response';
import { PERMISSION_SELECT } from '@/define';
import { CustomerRankServicesModel } from '@/core/models/config/customer/customer-rank-services.model';
import { CustomerRankServicesService } from '@/core/services/config/customer/customer-rank-services.service';
import { CustomerRankServicesForm } from '@/modules/config/config-customer/customer-rank-services/form/customer-rank-services-form';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-customer-rank-services-list',
  templateUrl: './customer-rank-services-list.html',
  imports: [FillHeightDirective, ...ListShareImports],
  standalone: true,
})
export class CustomerRankServicesList
  extends BaseListV2<CustomerRankServicesModel>
  implements OnInit
{
  modelService = inject(CustomerRankServicesService);
  @ViewChild('dt') dt!: Table<CustomerRankServicesModel>;
  displayCols: any = [
    { label: 'ID', value: 'id' },
    { label: 'Hạng thành viên', labelKey: 'common.label.rank', value: 'customerRanksInfo' },
    { label: '% giảm giá', labelKey: 'common.label.discount_percent', value: 'discount_percent' },
    { label: 'Dịch vụ', labelKey: 'common.label.service', value: 'servicesInfo' },
    { label: 'Trạng thái', labelKey: 'common.label.status', value: 'status' },
  ];
  override selectedColumns = signal(['id', 'customerRanksInfo', 'discount_percent', 'servicesInfo', 'status']);
  protected override LS_SELECTED_COLUMNS_KEY = '/config/customer-rank-services-list';
  protected override URL_STATE_PREFIX = 'crs';

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
   * Khai báo filter — chỉ có 1 ô tìm kiếm từ khoá.
   */
  protected filterFields(): FilterFieldDef[] {
    return [{ name: '_keyword', kind: 'text' }];
  }

  /**
   * Override `fetchList` chứ KHÔNG override `searchHandler`: base cho mọi lần tải đi qua một
   * stream có `switchMap` để huỷ request cũ, và tự lo `loading`/`models`/`paginationInfo`.
   */
  protected override fetchList(): Observable<ListApiResponse<CustomerRankServicesModel>> {
    return this.modelService
      .list({ params: this.searchParams() })
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

  checkUpdatePermission() {
    return !this.permissionsService.getPermission(
      PERMISSION_SELECT.permissionManageConfig,
    );
  }

  /**
   * Mở dialog thêm mới bản ghi
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
    const dialogRef = this.dialogWrapperService.open(CustomerRankServicesForm, {
      width: '700px',
      header,
    });
    dialogRef.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((result: CustomerRankServicesModel) => {
      if (result) {
        this.getList();
      }
    });
  }

  /**
   * Mở dialog cập nhật bản ghi
   */
  override updateRecord(model: CustomerRankServicesModel) {
    if (
      this.updatePermissionString &&
      !this.permissionsService.getPermission(this.updatePermissionString)
    ) {
      this.noti.warnMsg(this.translate.instant('common.message.no_permission'));
      return;
    }
    const header = this.translate.instant('common.action.update');
    const dialogRef = this.dialogWrapperService.open(CustomerRankServicesForm, {
      width: '700px',
      header,
      data: { model },
    });
    dialogRef.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((result: CustomerRankServicesModel) => {
      if (result) {
        const dataWithLabels = this.modelService.addLabel(result);
        this.updateRow(dataWithLabels as any);
      }
    });
  }
}
