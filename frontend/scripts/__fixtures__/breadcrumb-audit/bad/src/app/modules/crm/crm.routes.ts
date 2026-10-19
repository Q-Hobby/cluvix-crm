// @ts-nocheck
import {Routes} from '@angular/router';

export default [
  {
    path: '',
    loadComponent: () => import('@/pages/plain').then((c) => c.Plain),
    data: {breadcrumb: null},
  },
  {
    path: 'broker',
    data: {breadcrumb: 'x.list'},
    children: [
      {path: '', loadComponent: () => import('@/pages/plain').then((c) => c.Plain)},
      {
        path: 'detail/:id',
        loadComponent: () => import('@/pages/with-label').then((c) => c.WithLabel),
        data: {breadcrumb: 'x.detail'},
      },
      {
        // B6: có tham số nhưng component không gọi label service
        path: 'edit/:id',
        loadComponent: () => import('@/pages/plain').then((c) => c.Plain),
        data: {breadcrumb: 'x.detail'},
      },
      {
        // B5: trùng breadcrumb của cha ⇒ bị ẩn
        path: 'sub',
        loadComponent: () => import('@/pages/plain').then((c) => c.Plain),
        data: {breadcrumb: 'x.list'},
      },
    ],
  },
  {
    // B5 (F2): cha KHÔNG component, path khác rỗng > 'mid' không data (kế thừa x.list vì cha không component)
    // > lá 'g' khai trùng x.list ⇒ bị ẩn. Mô phỏng thiếu nhánh `!parent.hasComponent` ⇒ mất B5 này.
    path: 'plist',
    data: {breadcrumb: 'x.list'},
    children: [
      {path: '', loadComponent: () => import('@/pages/plain').then((c) => c.Plain)},
      {
        path: 'mid',
        children: [{path: 'g', loadComponent: () => import('@/pages/plain').then((c) => c.Plain), data: {breadcrumb: 'x.list'}}],
      },
    ],
  },
  {
    // B1: chuỗi thô
    path: 'raw',
    loadComponent: () => import('@/pages/plain').then((c) => c.Plain),
    data: {breadcrumb: 'Raw Label'},
  },
  {
    // B1: chữ hoa (kiểu PAGE_TITLE.LIST_APPOINTMENT)
    path: 'upper',
    loadComponent: () => import('@/pages/plain').then((c) => c.Plain),
    // data: {breadcrumb: 'Commented'},
    data: {breadcrumb: 'PAGE_TITLE.LIST'},
  },
  {
    // B1: đúng dạng key nhưng không có trong vi-VN.json
    path: 'missing',
    loadComponent: () => import('@/pages/plain').then((c) => c.Plain),
    data: {breadcrumb: 'x.missing'},
  },
  {
    // B3: có tham số nhưng chỉ 2 crumb (anh em phẳng, không có cấp danh sách)
    path: 'flat/:id',
    loadComponent: () => import('@/pages/with-label').then((c) => c.WithLabel),
    data: {breadcrumb: 'x.detail'},
  },
  {
    // B4: routerLink trỏ URL không có lá nào
    path: 'convert',
    data: {breadcrumb: 'x.list', routerLink: '/crm/nope'},
    children: [
      {
        path: 'detail/:id',
        loadComponent: () => import('@/pages/with-label').then((c) => c.WithLabel),
        data: {breadcrumb: 'x.detail'},
      },
    ],
  },
] as Routes;
