import {ChangeDetectionStrategy, Component, computed, effect, inject, model, OnInit, signal, ViewChild} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Table } from 'primeng/table';

import { ConfigCodeForm } from '@/modules/config/config-system/config-code/form/config-code-form';
import { ConfigCodeModel, CustomerCodeOptionInfo } from '@/core/models/config/system/config-code.model';
import { ConfigCodeService } from '@/core/services/config/system/config-code.service';
import { BaseListV2 } from '@/share/base/base-list-v2';
import { FilterFieldDef } from '@/share/base/list-filter.model';
import { CONFIG_CODE_PARENT_TABLE_CUSTOMER, PERMISSION_SELECT } from '@/define';
import { ListShareImports } from '@/share/import/list-share-imports';
import { Observable, switchMap } from 'rxjs';
import { ListApiResponse } from '@/share/models/list-api-response';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-config-code-list',
  templateUrl: './config-code-list.html',
  styles: [],
  imports: [FillHeightDirective, ...ListShareImports],
  standalone: true,
})
export class ConfigCodeList extends BaseListV2<ConfigCodeModel> implements OnInit {
  modelService = inject(ConfigCodeService);
  @ViewChild('dt') dt!: Table<ConfigCodeModel>;
  protected override URL_STATE_PREFIX = 'cco';
  override formComponent = ConfigCodeForm;  override updatePermissionString = PERMISSION_SELECT.permissionManageConfig;
  override createPermissionString = PERMISSION_SELECT.permissionManageConfig;  displayCols: any = [
    { label: 'ID', labelKey: 'common.label.id', value: 'id' },
    { label: 'Định danh', labelKey: 'common.label.identifier', value: 'name' },
    { label: 'Chi nhánh', labelKey: 'common.label.branch', value: 'branch' },
    { label: 'Table', value: 'parent_table' },
    { label: 'Cấu hình', labelKey: 'config.config_code.col_config', value: 'config' },
    { label: 'Last number', value: 'lastNumber' },
    { label: 'Next', value: 'next' },
  ];
  override selectedColumns = signal([
    'id',
    'name',
    'branch',
    'parent_table',
    'config',
    'lastNumber',
    'next',
  ]);

  constructor() {
    super();
    effect(() => {
      const len = this.selectedColumns()?.length ?? 0;
      this.padCols.set(len + 2);
    });
  }

  /** Kiểu mã khách BE đang chạy; khác mặc định ⇒ dòng "customer" không còn quyết định định dạng mã. */
  readonly customerCodeOption = signal<CustomerCodeOptionInfo | null>(null);
  readonly customerOptionOverridden = computed(() => {
    const info = this.customerCodeOption();
    return !!info && info.option !== info.default_option;
  });

  ngOnInit() {
    this.initListV2();
    this.modelService
      .getCustomerCodeOption()
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe((info) => this.customerCodeOption.set(info));
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
  protected override fetchList(): Observable<ListApiResponse<ConfigCodeModel>> {
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

  nextCodePreview(model: ConfigCodeModel): string {
    // Định dạng mã khách do CUSTOMER_CODE_OPTION quyết định ⇒ xem trước theo cấu hình dòng này là SAI.
    if (model.parent_table === CONFIG_CODE_PARENT_TABLE_CUSTOMER && this.customerOptionOverridden()) {
      return '—';
    }
    const parts: string[] = [];
    if (model.include_branch_code !== false && (model as any).branchInfo?.code) {
      parts.push((model as any).branchInfo.code);
    }
    if (model.infix_code) parts.push(model.infix_code);
    const prefix = parts.length ? parts.join('.') + '.' : '';

    const lastNumber = (model as any).configCodeMonthly?.last_number ?? 0;
    const nextSerial = lastNumber + 1;
    const minWidth = Math.max(model.serial_length ?? 2, 1);
    const serialStr = String(nextSerial).padStart(minWidth, '0');

    if (model.monthly_reset === false) {
      return (prefix + serialStr).toUpperCase();
    }
    const now = new Date();
    const ym = String(now.getFullYear()).slice(-2) + String(now.getMonth() + 1).padStart(2, '0');
    return (prefix + ym + '.' + serialStr).toUpperCase();
  }

  override updateRecord(model: ConfigCodeModel) {
    if (
        this.updatePermissionString &&
        !this.permissionsService.getPermission(this.updatePermissionString)
    ) {
      this.noti.warnMsg(this.translate.instant('common.message.no_permission'));
      return;
    }
    const header = this.translate.instant('common.action.update');
    const dialogRef = this.dialogWrapperService.open(this.formComponent, {
      width: '700px',
      header,
      data: { model },
    });

    dialogRef.onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((res: any) => {
      if (res) {
        const dataWithLabels = this.modelService.addLabel(res);
        this.updateRow(dataWithLabels as any);
      }
    });
  }
}
