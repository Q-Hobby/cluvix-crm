import {ChangeDetectionStrategy, Component, effect, inject, OnInit, signal, ViewChild } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Table } from 'primeng/table';

import { SourceForm } from '@/modules/config/config-customer/source/form/source-form';
import { SourceModel } from '@/core/models/config/marketing/source.model';
import { SourceService } from '@/core/services/config/marketing/source.service';
import { BaseListV2 } from '@/share/base/base-list-v2';
import { FilterFieldDef } from '@/share/base/list-filter.model';
import { PERMISSION_SELECT } from '@/define';
import { ListShareImports } from '@/share/import/list-share-imports';
import { Observable, switchMap } from 'rxjs';
import { ListApiResponse } from '@/share/models/list-api-response';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-source-list',
  templateUrl: './source-list.html',
  styles: [],
  imports: [FillHeightDirective, ...ListShareImports],
  standalone: true,
})
export class SourceList extends BaseListV2<SourceModel> implements OnInit {
  modelService = inject(SourceService);
  @ViewChild('dt') dt!: Table<SourceModel>;
  protected override URL_STATE_PREFIX = 'so';
  displayCols: any = [
    { label: 'ID', value: 'id' },
    { label: 'Nguồn khách hàng', labelKey: 'common.label.source', value: 'name' },
    { label: 'Trạng thái', labelKey: 'common.label.status', value: 'status' },
    { label: 'Vị trí', labelKey: 'common.label.sort_order', value: 'SortOrder' },
  ];
  override selectedColumns = signal(['id', 'name', 'status', 'SortOrder']);
  protected override LS_SELECTED_COLUMNS_KEY = '/config/source-list';

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
   * Khai báo filter — list này chỉ có ô tìm kiếm theo từ khoá.
   */
  protected filterFields(): FilterFieldDef[] {
    return [{ name: '_keyword', kind: 'text' }];
  }

  protected override fetchList(): Observable<ListApiResponse<SourceModel>> {
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
    const dialogRef = this.dialogWrapperService.open(SourceForm, {
      width: '700px',
      header,
    });
    dialogRef.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((result: SourceModel) => {
      if (result) {
        this.getList();
      }
    });
  }

  /**
   * Mở dialog cập nhật bản ghi
   */
  override updateRecord(model: SourceModel) {
    if (
      this.updatePermissionString &&
      !this.permissionsService.getPermission(this.updatePermissionString)
    ) {
      this.noti.warnMsg(this.translate.instant('common.message.no_permission'));
      return;
    }
    const header = this.translate.instant('common.action.update');
    const dialogRef = this.dialogWrapperService.open(SourceForm, {
      width: '700px',
      header,
      data: { model },
    });
    dialogRef.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((result: SourceModel) => {
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
