import {ChangeDetectionStrategy, Component, effect, inject, OnInit, signal, ViewChild } from '@angular/core';
import { ListShareImports } from '@/share/import/list-share-imports';
import { BaseListV2 } from '@/share/base/base-list-v2';
import { FilterFieldDef } from '@/share/base/list-filter.model';
import { CustomerRanksModel } from '@/core/models/config/customer/customer-ranks.model';
import { CustomerRanksService } from '@/core/services/config/customer/customer-ranks.service';
import { Table } from 'primeng/table';
import { PERMISSION_SELECT } from '@/define';
import { CustomerRanksForm } from '@/modules/config/config-customer/customer-ranks/form/customer-ranks-form';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-customer-ranks-list',
  templateUrl: './customer-ranks-list.html',
  imports: [FillHeightDirective, ...ListShareImports],
  standalone: true,
})
export class CustomerRanksList extends BaseListV2<CustomerRanksModel> implements OnInit {
  modelService = inject(CustomerRanksService);
  @ViewChild('dt') dt!: Table<CustomerRanksModel>;
  displayCols: any = [
    { label: 'ID', value: 'id' },
    { label: 'Hạng thành viên', labelKey: 'common.label.rank', value: 'name' },
    { label: 'Điểm tối thiểu', labelKey: 'common.label.min_points', value: 'min_points' },
    { label: 'Điểm tối đa', labelKey: 'common.label.max_points', value: 'max_points' },
    { label: '% giảm giá', labelKey: 'common.label.discount_percent', value: 'discount_percent' },
    { label: 'Màu hiển thị', labelKey: 'common.label.display_color', value: 'color_code' },
    { label: 'Trạng thái', labelKey: 'common.label.status', value: 'status' },
    { label: 'Mô tả', labelKey: 'common.label.description', value: 'description' },
  ];
  override selectedColumns = signal(['id', 'name', 'min_points', 'max_points', 'discount_percent', 'color_code', 'status', 'description']);
  protected override LS_SELECTED_COLUMNS_KEY = '/config/customer-ranks-list';
  protected override URL_STATE_PREFIX = 'cr';

  /** START CRUD */
  override formComponent = CustomerRanksForm;  override updatePermissionString = PERMISSION_SELECT.permissionManageConfig;
  override createPermissionString = PERMISSION_SELECT.permissionManageConfig;
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

  // Không override fetchList(): default của BaseListV2 (modelService.list({params: searchParams()}))
  // khớp đúng hành vi V1 — addLabels ở đây vốn đã bị comment-out (chết) từ trước khi migrate.

  checkUpdatePermission() {
    return !this.permissionsService.getPermission(PERMISSION_SELECT.permissionManageConfig);
  }
}
