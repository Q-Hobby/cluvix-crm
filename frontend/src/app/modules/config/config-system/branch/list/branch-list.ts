import {ChangeDetectionStrategy, Component, inject, OnInit, signal, ViewChild, effect} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Table } from 'primeng/table';

import { BranchForm } from '@/modules/config/config-system/branch/form/branch-form';
import { BranchModel } from '@/core/models/config/system/branch.model';
import { BranchService } from '@/core/services/config/system/branch.service';
import { BaseListV2 } from '@/share/base/base-list-v2';
import { FilterFieldDef } from '@/share/base/list-filter.model';
import { PERMISSION_SELECT } from '@/define';
import { ListShareImports } from '@/share/import/list-share-imports';
import { Observable, switchMap } from 'rxjs';
import { ListApiResponse } from '@/share/models/list-api-response';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-branch-list',
  templateUrl: './branch-list.html',
  styles: [],
  imports: [FillHeightDirective, ...ListShareImports],
  standalone: true,
})
export class BranchList extends BaseListV2<BranchModel> implements OnInit {
  modelService = inject(BranchService);
  @ViewChild('dt') dt!: Table<BranchModel>;
  protected override URL_STATE_PREFIX = 'br';
  options!: number;
  displayCols: any = [
    { label: 'ID', value: 'id' },
    { label: 'Chi nhánh', labelKey: 'common.label.branch', value: 'name' },
    { label: 'Mã', labelKey: 'common.label.code', value: 'code' },
    { label: 'Email', labelKey: 'common.label.email', value: 'email' },
    { label: 'Lịch hẹn', labelKey: 'common.label.appointment', value: 'statusOpenAppointment' },
    { label: 'Trạng thái', labelKey: 'common.label.status', value: 'status' },
    { label: 'Vị trí', labelKey: 'common.label.sort_order', value: 'SortOrder' },
  ];
  override selectedColumns = signal(['id', 'name', 'code', 'email', 'statusOpenAppointment', 'status', 'SortOrder']);
  protected override LS_SELECTED_COLUMNS_KEY = '/config/branch-list';

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
   * Override `fetchList` chứ KHÔNG override `searchHandler`: base cho mọi lần tải đi qua một
   * stream có `switchMap` để huỷ request cũ, và tự lo `loading`/`models`/`paginationInfo`.
   */
  protected override fetchList(): Observable<ListApiResponse<BranchModel>> {
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
    const dialogRef = this.dialogWrapperService.open(BranchForm, {
      width: '960px',
      header,
    });
    dialogRef.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((result: BranchModel) => {
      if (result) {
        this.getList();
      }
    });
  }

  /**
   * Mở dialog cập nhật bản ghi
   */
  override updateRecord(model: BranchModel) {
    if (
      this.updatePermissionString &&
      !this.permissionsService.getPermission(this.updatePermissionString)
    ) {
      this.noti.warnMsg(this.translate.instant('common.message.no_permission'));
      return;
    }
    const header = this.translate.instant('common.action.update');
    const dialogRef = this.dialogWrapperService.open(BranchForm, {
      width: '960px',
      header,
      data: { model },
    });
    dialogRef.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((result: BranchModel) => {
      if (result) {
        this.getList();
      }
    });
  }

  /**
   * Xử lý thay đổi trạng thái (Nhận khách/Không nhận khách) của chi nhánh
   */
  handleStatusOpenAppointmentChange(id: number) {
    this.modelService.showHideOpenAppointment({ id }).subscribe((res: any) => {
      this.noti.successRes(res);
    });
  }

  checkUpdatePermission() {
    return !this.permissionsService.getPermission(PERMISSION_SELECT.permissionManageConfig);
  }
}
