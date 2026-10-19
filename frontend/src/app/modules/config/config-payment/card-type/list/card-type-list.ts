import {ChangeDetectionStrategy, Component, effect, inject, OnInit, signal, ViewChild } from '@angular/core';
import { Table } from 'primeng/table';

import { CardTypeForm } from '@/modules/config/config-payment/card-type/form/card-type-form';
import { CardTypeModel } from '@/core/models/config/accountant/card-type.model';
import { CardTypeService } from '@/core/services/config/accountant/card-type.service';
import { BaseListV2 } from '@/share/base/base-list-v2';
import { FilterFieldDef } from '@/share/base/list-filter.model';
import { PERMISSION_SELECT } from '@/define';
import { ListShareImports } from '@/share/import/list-share-imports';
import { Observable, switchMap } from 'rxjs';
import { ListApiResponse } from '@/share/models/list-api-response';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-card-type-list',
  templateUrl: './card-type-list.html',
  styles: [],
  imports: [FillHeightDirective, ...ListShareImports],
  standalone: true,
})
export class CardTypeList extends BaseListV2<CardTypeModel> implements OnInit {
  modelService = inject(CardTypeService);
  @ViewChild('dt') dt!: Table<CardTypeModel>;
  displayCols: any = [
    { label: 'ID', labelKey: 'common.label.id', value: 'id' },
    { label: 'Tên', labelKey: 'common.label.name', value: 'name' },
    { label: 'Ghi chú', labelKey: 'config.card_type.extra_description', value: 'note' },
    { label: 'Phương thức', labelKey: 'common.label.payment_method', value: 'paymentMethod' },
    { label: 'Trạng thái', labelKey: 'common.label.status', value: 'status' },
    { label: 'Vị trí', labelKey: 'common.label.sort_order', value: 'SortOrder' },
  ];
  /** START CRUD */
  override formComponent = CardTypeForm;  override updatePermissionString = PERMISSION_SELECT.permissionManageConfig;
  override createPermissionString = PERMISSION_SELECT.permissionManageConfig;  override selectedColumns = signal(['id', 'name', 'note', 'paymentMethod', 'status', 'SortOrder']);
  protected override LS_SELECTED_COLUMNS_KEY = '/config/app-card-type-list';
  protected override URL_STATE_PREFIX = 'ct';

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
   * Khai báo filter — chỉ `_keyword`. Control `method` khai trong `fb.group` cũ nhưng KHÔNG có
   * `formControlName="method"` nào trong template (grep xác nhận rỗng) và BE `ListCardType`
   * (internal/modules/webapp/config/card_type/handler/handler.go) không đọc field `method` ở
   * bất kỳ dạng nào (ApplyFilter/GetFilterValues/c.Query) → control chết, không khai def.
   */
  protected filterFields(): FilterFieldDef[] {
    return [
      {name: '_keyword', kind: 'text'},
    ];
  }

  /**
   * Override `fetchList` chứ KHÔNG override `searchHandler`: base cho mọi lần tải đi qua một
   * stream có `switchMap` để huỷ request cũ, và tự lo `loading`/`models`/`paginationInfo`.
   */
  protected override fetchList(): Observable<ListApiResponse<CardTypeModel>> {
    return this.modelService.list({params: this.searchParams()}).pipe(
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
}
