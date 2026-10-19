import {ChangeDetectionStrategy, Component, DestroyRef, effect, inject, OnDestroy, OnInit, signal, ViewChild} from '@angular/core';
import {takeUntilDestroyed} from '@angular/core/rxjs-interop';
import {Table} from 'primeng/table';
import {Observable, finalize, from, switchMap} from 'rxjs';

import {FormInUnit} from '@/modules/config/config-system/unit/form/form-in-unit/form-in-unit';
import {UnitModel} from '@/core/models/config/system/unit.model';
import {UnitService} from '@/core/services/config/system/unit.service';
import {BaseListV2} from '@/share/base/base-list-v2';
import {FilterFieldDef} from '@/share/base/list-filter.model';
import {PERMISSION_SELECT, UNIT_CATEGORY_OPTIONS} from '@/define';
import {ListShareImports} from '@/share/import/list-share-imports';
import {ListApiResponse} from '@/share/models/list-api-response';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-unit-list',
  templateUrl: './unit-list.html',
  styles: [],
  imports: [FillHeightDirective, ...ListShareImports],
  standalone: true,
})
export class UnitList extends BaseListV2<UnitModel> implements OnInit, OnDestroy {
  modelService = inject(UnitService);
  @ViewChild('dt') dt!: Table<UnitModel>;
  protected override URL_STATE_PREFIX = 'un';
  displayCols: any = [
    { label: 'ID', value: 'id' },
    { label: 'Tên đơn vị', labelKey: 'common.label.unit_name', value: 'name' },
    { label: 'Mô tả', labelKey: 'common.label.description', value: 'description' },
    { label: 'Trạng thái', labelKey: 'common.label.status', value: 'status' },
    { label: 'Vị trí', labelKey: 'common.label.sort_order', value: 'SortOrder' },
  ];
  override selectedColumns = signal(['id', 'name', 'description', 'status', 'SortOrder']);
  protected override LS_SELECTED_COLUMNS_KEY = '/config/unit-list';

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
   * Override `fetchList` chứ KHÔNG override `searchHandler`: base cho mọi lần tải đi qua một
   * stream có `switchMap` để huỷ request cũ, và tự lo `loading`/`models`/`paginationInfo`.
   */
  protected override fetchList(): Observable<ListApiResponse<UnitModel>> {
    return this.modelService.list({params: this.searchParams()}).pipe(
      switchMap((response: any) =>
        from(this.modelService.addLabels(response.items ?? [])).pipe(
          switchMap((items) => [{...response, items}])
        )
      ),
    );
  }

  override createRecord(): void {
    if (this.createPermissionString && !this.permissionsService.getPermission(this.createPermissionString)) {
      this.noti.warnMsg(this.translate.instant('common.message.no_permission'));
      return;
    }
    this.dialogWrapperService.open(FormInUnit, {
      width: '840px',
      header: this.createDialogTitleString || this.translate.instant('common.action.create'),
    }).onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((result: UnitModel) => {
      if (result) this.getList();
    });
  }

  override updateRecord(model: UnitModel) {
    if (this.updatePermissionString && !this.permissionsService.getPermission(this.updatePermissionString)) {
      this.noti.warnMsg(this.translate.instant('common.message.no_permission'));
      return;
    }
    this.dialogWrapperService.open(FormInUnit, {
      width: '840px',
      header: this.updateDialogTitleString || this.translate.instant('common.action.update'),
      data: {model},
    }).onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((result: UnitModel) => {
      if (result) this.updateRow(this.modelService.addLabel(result));
    });
  }

  /**
   * Khai báo filter. `unitCategory_id`: template `p-select [multiple]="true"` +
   * `fb.control([])` (default mảng rỗng) + matchMode `'in'` cũ → đúng là multiselect thật
   * (không phải dropdown chọn-một trông giống), giữ nguyên `kind: 'in'`.
   * BE `unit.List` (internal/modules/webapp/config/unit/handler/handler.go:59) đọc field này
   * qua `c.Query("filter[unitCategory_id]")` (mẫu ĐÔNG NHẤT ở BE config) → có def, khai bình
   * thường; `ApplyFilter`/`GetFilterValues`/`ApplyKeywordFilter`/`c.Query("unitCategory_id`
   * không có hit nào, chỉ hit đúng mẫu thứ 5.
   */
  protected filterFields(): FilterFieldDef[] {
    return [
      { name: '_keyword', kind: 'text' },
      { name: 'unitCategory_id', kind: 'in' },
    ];
  }

  exportUnit(event: Event) {
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
        this.loadingService.showLoading('exportUnit');
        const params = {...this.exportParams()} as any;
        this.modelService.exportUnit({params}).pipe(
          finalize(() => {
            this.isSubmitting.set(false);
            this.loadingService.hideLoading('exportUnit');
          }),
        ).subscribe({
          next: (res: any) => {
            this.noti.successMsg(this.translate.instant('common.message.export_success'));
            this.downloadExport(res?.url || res);
          },
        });
      },
      reject: () => this.isSubmitting.set(false),
    });
  }

  checkUpdatePermission() {
    return !this.permissionsService.getPermission(PERMISSION_SELECT.permissionManageConfig);
  }

  protected readonly UNIT_CATEGORY_OPTIONS = UNIT_CATEGORY_OPTIONS;
}
