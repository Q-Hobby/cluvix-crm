import { Routes } from '@angular/router';

// story-20: relocated từ config-accoutant (đã xoá — accountant module bỏ theo architecture.md
// §1). card_type/payment_method/bank_account là BE `webapp/config` keep-list, không thuộc module
// accountant — chỉ từng bị gom nhầm thư mục.
export default [
  {
    path: 'card-type',
    loadComponent: () =>
      import('./card-type/list/card-type-list').then((c) => c.CardTypeList),
    data: { breadcrumb: 'config.nav.account.payment_method' },
  },
  {
    path: 'payment-method',
    loadComponent: () =>
      import('./payment-method/list/payment-method-list').then((c) => c.PaymentMethodList),
    data: {
      breadcrumb: 'config.nav.account.payment_form',
    },
  },
  {
    path: 'bank-account',
    loadComponent: () =>
      import('./bank-account/list/bank-account-list').then((c) => c.BankAccountList),
    data: {
      breadcrumb: 'config.nav.account.bank_account',
    },
  },
] as Routes;
