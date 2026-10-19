import {ChangeDetectionStrategy, Component, effect, inject, OnInit, signal, ViewChild} from '@angular/core';
import {takeUntilDestroyed} from '@angular/core/rxjs-interop';
import {Table} from 'primeng/table';
import {from, Observable, switchMap} from 'rxjs';

import {BankAccountForm} from '@/modules/config/config-payment/bank-account/form/bank-account-form';
import {BankAccountModel} from '@/core/models/config/accountant/bank-account.model';
import {BankAccountService} from '@/core/services/config/accountant/bank-account.service';
import {BaseListV2} from '@/share/base/base-list-v2';
import {FilterFieldDef} from '@/share/base/list-filter.model';
import {ListApiResponse} from '@/share/models/list-api-response';
import {PERMISSION_SELECT, STATUS_OPTION} from '@/define';
import {ListShareImports} from '@/share/import/list-share-imports';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-bank-account-list',
  templateUrl: './bank-account-list.html',
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
export class BankAccountList extends BaseListV2<BankAccountModel> implements OnInit {
  modelService = inject(BankAccountService);
  @ViewChild('dt') dt!: Table<BankAccountModel>;
  displayCols: any = [
    { label: 'ID', value: 'id' },
    { label: 'Mã ngân hàng', labelKey: 'config.bank_account.bank_code', value: 'bank_code' },
    { label: 'Số tài khoản', labelKey: 'common.label.account_number', value: 'account_no' },
    { label: 'Tên chủ tài khoản', labelKey: 'config.bank_account.account_name', value: 'account_name' },
    { label: 'Trạng thái', labelKey: 'common.label.status', value: 'status' },
  ];
  override selectedColumns = signal(['id', 'bank_code', 'account_no', 'account_name', 'status']);
  protected override LS_SELECTED_COLUMNS_KEY = '/config/bank-account';
  protected override URL_STATE_PREFIX = 'ba';
  readonly canUpdate = !this.permissionsService.getPermission(PERMISSION_SELECT.permissionManageConfig);

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
   * Khai báo filter — base tự sinh FormGroup, đấu valueChanges → dt.filter(),
   * dựng filters khởi tạo cho p-table, lo clearFilter() và đồng bộ URL.
   * `status` là multiselect thật trong V1 (fb.control([]), dt.filter matchMode 'in',
   * `p-select [multiple]="true"`) → giữ nguyên kind: 'in'.
   */
  protected filterFields(): FilterFieldDef[] {
    return [
      {name: '_keyword', kind: 'text'},
      {name: 'status', kind: 'in'},
    ];
  }

  /**
   * Override `fetchList` chứ KHÔNG override `searchHandler`: base cho mọi lần tải đi qua một
   * stream có `switchMap` để huỷ request cũ, và tự lo `loading`/`models`/`paginationInfo`.
   */
  protected override fetchList(): Observable<ListApiResponse<BankAccountModel>> {
    return this.modelService.list({params: this.searchParams()}).pipe(
      switchMap((response: any) =>
        from(this.modelService.addLabels(response.items ?? [])).pipe(
          switchMap((items) => [{...response, items} as ListApiResponse<BankAccountModel>])
        )
      ),
    );
  }

  createBankAccount() {
    if (!this.permissionsService.getPermission(PERMISSION_SELECT.permissionManageConfig)) {
      this.noti.warnMsg(this.translate.instant('common.message.no_permission'));
      return;
    }
    this.dialogWrapperService.open(BankAccountForm, {
      width: '840px',
      header: this.translate.instant('config.accoutant_bank_account.dialog_create'),
    }).onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((result: any) => {
      if (result) this.getList();
    });
  }

  updateBankAccount(model: BankAccountModel) {
    if (!this.permissionsService.getPermission(PERMISSION_SELECT.permissionManageConfig)) {
      this.noti.warnMsg(this.translate.instant('common.message.no_permission'));
      return;
    }
    this.dialogWrapperService.open(BankAccountForm, {
      width: '840px',
      header: this.translate.instant('config.accoutant_bank_account.dialog_update'),
      data: {model, clone: false},
    }).onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((result: BankAccountModel) => {
      if (result) this.getList();
    });
  }

  cloneBankAccount(model: BankAccountModel) {
    if (!this.permissionsService.getPermission(PERMISSION_SELECT.permissionManageConfig)) {
      this.noti.warnMsg(this.translate.instant('common.message.no_permission'));
      return;
    }
    this.dialogWrapperService.open(BankAccountForm, {
      width: '840px',
      header: this.translate.instant('config.accoutant_bank_account.dialog_create_2'),
      data: {model, clone: true},
    }).onClose.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((result: any) => {
      if (result) this.getList();
    });
  }

  protected readonly STATUS_OPTION = STATUS_OPTION;
}
