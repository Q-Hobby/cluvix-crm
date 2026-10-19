import {ChangeDetectionStrategy, Component, effect, inject, OnInit, signal, ViewChild } from '@angular/core';
import { Table } from 'primeng/table';
import { Observable } from 'rxjs';

import { PaymentMethodForm } from '@/modules/config/config-payment/payment-method/form/payment-method-form';
import { PaymentMethodModel } from '@/core/models/config/accountant/payment-method.model';
import { PaymentMethodService } from '@/core/services/config/accountant/payment-method.service';
import { BaseListV2 } from '@/share/base/base-list-v2';
import { FilterFieldDef } from '@/share/base/list-filter.model';
import { ListApiResponse } from '@/share/models/list-api-response';
import { PERMISSION_SELECT } from '@/define';
import { ListShareImports } from '@/share/import/list-share-imports';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-payment-method-list',
  templateUrl: './payment-method-list.html',
  styles: [],
  imports: [FillHeightDirective, ...ListShareImports],
  standalone: true,
})
export class PaymentMethodList extends BaseListV2<PaymentMethodModel> implements OnInit {
  @ViewChild('dt') dt!: Table<PaymentMethodModel>;
  displayCols: any = [
    { label: 'ID', labelKey: 'common.label.id', value: 'id' },
    { label: 'Phương thức thanh toán', labelKey: 'common.label.payment_method', value: 'name' },
    { label: 'Trạng thái', labelKey: 'common.label.status', value: 'status' },
    { label: 'Vị trí', labelKey: 'common.label.sort_order', value: 'SortOrder' },
  ];
  override selectedColumns = signal(['id', 'name', 'status', 'SortOrder']);
  protected override URL_STATE_PREFIX = 'pm';

  /** START CRUD */
  override formComponent = PaymentMethodForm;  override updatePermissionString = PERMISSION_SELECT.permissionManageConfig;
  override createPermissionString = PERMISSION_SELECT.permissionManageConfig;  modelService = inject(PaymentMethodService);

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

  /** Chỉ có `_keyword`. */
  protected filterFields(): FilterFieldDef[] {
    return [{name: '_keyword', kind: 'text'}];
  }

  /**
   * Override `fetchList` chứ KHÔNG override `searchHandler`: base cho mọi lần tải đi qua một
   * stream có `switchMap` để huỷ request cũ, và tự lo `loading`/`models`/`paginationInfo`.
   */
  protected override fetchList(): Observable<ListApiResponse<PaymentMethodModel>> {
    return this.modelService.list({params: this.searchParams()});
  }

  checkUpdatePermission() {
    return !this.permissionsService.getPermission(
      PERMISSION_SELECT.permissionManageConfig,
    );
  }
}
