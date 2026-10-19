import { Routes } from '@angular/router';

// story-20: relocated từ config-hr (đã xoá — module hr/config-hr bỏ theo architecture.md §1).
// `hr_department` là NGOẠI LỆ DUY NHẤT giữ lại trong `webapp/hr`/`webapp/config` (D8 — telesale
// scope cần user_profile.departmentList). BE hiện CÒN đủ CRUD `/config/hr-department` (GET list/
// detail, POST, PUT, DELETE — xem backend/internal/modules/webapp/config/hr_department/route.go)
// nên giữ nguyên màn CRUD, không rút gọn xuống lookup-only.
export default [
  {
    path: 'departments',
    loadComponent: () =>
      import('@/modules/config/config-department/department/list/department-list').then((c) => c.DepartmentList),
    data: { breadcrumb: 'common.label.department' },
  },
] as Routes;
