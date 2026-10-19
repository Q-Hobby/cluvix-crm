import {ChangeDetectionStrategy, Component, effect, inject, OnInit, signal, ViewChild } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Table } from 'primeng/table';
import { Observable, finalize, switchMap } from 'rxjs';

import { ServiceTypeForm } from '@/modules/config/config-clinic/service-type/form/service-type-form';
import { ServiceTypeModel } from '@/core/models/config/clinic/service-type.model';
import { ServiceTypeService } from '@/core/services/config/clinic/service-type.service';
import { BaseListV2 } from '@/share/base/base-list-v2';
import { FilterFieldDef } from '@/share/base/list-filter.model';
import { ListApiResponse } from '@/share/models/list-api-response';
import { PERMISSION_SELECT } from '@/define';
import { ListShareImports } from '@/share/import/list-share-imports';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-service-type-list',
  templateUrl: './service-type-list.html',
  styles: [
    `
      @media screen and (max-width: 40em) {
        :host ::ng-deep.p-menu.p-menu-overlay {
          top: 22px !important;
          left: 18px !important;
        }
      }
    `,
  ],
  imports: [FillHeightDirective, ...ListShareImports],
  standalone: true,
})
export class ServiceTypeList extends BaseListV2<ServiceTypeModel> implements OnInit {
  modelService = inject(ServiceTypeService);
  @ViewChild('dt') dt!: Table;
  displayCols: any = [
    { label: 'ID', labelKey: 'common.label.id', value: 'id' },
    { label: 'Nhóm dịch vụ', labelKey: 'common.label.service_group', value: 'name' },
    { label: 'Trạng thái', labelKey: 'common.label.status', value: 'status' },
    { label: 'Vị trí', labelKey: 'common.label.sort_order', value: 'SortOrder' },
  ];
  override selectedColumns = signal(['id', 'name', 'status', 'SortOrder']);
  protected override LS_SELECTED_COLUMNS_KEY = '/config/service-type-list';
  override createPermissionString = PERMISSION_SELECT.permissionManageConfig;
  override updatePermissionString = PERMISSION_SELECT.permissionManageConfig;
  protected override URL_STATE_PREFIX = 'st';

  constructor() {
    super();
    effect(() => {
      const len = this.selectedColumns()?.length ?? 0;
      this.padCols.set(len + 3);
    });
  }

  ngOnInit() {
    this.initListV2();
  }

  /**
   * Khai báo filter — chỉ có ô tìm kiếm tự do.
   */
  protected filterFields(): FilterFieldDef[] {
    return [{ name: '_keyword', kind: 'text' }];
  }

  /**
   * Override `fetchList` chứ KHÔNG override `searchHandler`: base cho mọi lần tải đi qua một
   * stream có `switchMap` để huỷ request cũ, và tự lo `loading`/`models`/`paginationInfo`.
   */
  protected override fetchList(): Observable<ListApiResponse<ServiceTypeModel>> {
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

  override createRecord(): void {
    if (
      this.createPermissionString &&
      !this.permissionsService.getPermission(this.createPermissionString)
    ) {
      this.noti.warnMsg(this.translate.instant('common.message.no_permission'));
      return;
    }
    const header =
      this.createDialogTitleString ||
      this.translate.instant('common.action.create');
    const dialogRef = this.dialogWrapperService.open(ServiceTypeForm, {
      width: '700px',
      header,
    });
    dialogRef.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((result: ServiceTypeModel) => {
      if (result) {
        this.getList();
      }
    });
  }

  /**
   * Mở dialog cập nhật bản ghi
   */
  override updateRecord(model: ServiceTypeModel) {
    if (
      this.updatePermissionString &&
      !this.permissionsService.getPermission(this.updatePermissionString)
    ) {
      this.noti.warnMsg(this.translate.instant('common.message.no_permission'));
      return;
    }
    const header =
      this.updateDialogTitleString ||
      this.translate.instant('common.action.update');
    const dialogRef = this.dialogWrapperService.open(ServiceTypeForm, {
      width: '700px',
      header,
      data: { model },
    });
    dialogRef.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((result: ServiceTypeModel) => {
      if (result) {
        // Vì có upload file nên không dùng updateRow được
        this.getList();
      }
    });
  }

  exportServiceType(event: Event) {
    if (!this.permissionsService.getPermission(PERMISSION_SELECT.permissionManageConfig)) {
      this.noti.warnMsg(this.translate.instant('common.message.no_permission'));
      return;
    }
    this.isSubmitting.set(true);
    this.confirmationService.confirm({
      target: (event.target ?? undefined),
      message: this.translate.instant('common.message.confirm_export'),
      icon: 'pi pi-exclamation-triangle',
      acceptButtonProps: {severity: 'danger', size: 'small'},
      accept: () => {
        this.loadingService.showLoading('exportServiceType');
        const params = {...this.exportParams()} as any;
        this.modelService
          .exportServiceType({
            params,
          })
          .pipe(
            finalize(() => {
              this.isSubmitting.set(false);
              this.loadingService.hideLoading('exportServiceType');
            }),
          )
          .subscribe({
            next: (res: any) => {
              this.noti.successMsg(this.translate.instant('common.message.export_success'));
              this.downloadExport(res);
            },
          });
      },
      reject: () => {
        this.isSubmitting.set(false);
      },
    });
  }

  checkUpdatePermission() {
    return !this.permissionsService.getPermission(PERMISSION_SELECT.permissionManageConfig);
  }
}
