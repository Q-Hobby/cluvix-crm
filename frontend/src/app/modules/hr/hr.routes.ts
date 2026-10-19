import { Routes } from '@angular/router';
import { PERMISSION_SELECT } from '@/define';
import { requirePerm } from '@/core/guards/has-perm.guard';

// story-20: modules/hr chỉ còn user + role (D8 — phòng ban vẫn quản lý được qua
// /config/departments, xem modules/config/config-department). Dashboard/attendance/schedule/
// leave/ot-request/payroll/comp/contract/certificate/partner đã xoá theo architecture.md §1.
export default [
  {
    path: '',
    redirectTo: 'users',
    pathMatch: 'full',
  },
  {
    path: 'users',
    canActivate: [requirePerm(PERMISSION_SELECT.permissionViewUser)],
    loadComponent: () => import('@/modules/hr/user/user-list/user-list').then((c) => c.UserList),
    data: { breadcrumb: 'common.label.staff_list', title: 'common.label.staff_list' },
  },
  {
    path: 'roles',
    canActivate: [requirePerm(PERMISSION_SELECT.permissionViewRole)],
    loadComponent: () =>
      import('@/modules/hr/role/role-user-list/user-role').then((c) => c.UserRole),
    data: { breadcrumb: 'common.label.role', title: 'common.label.role' },
  },
] as Routes;
