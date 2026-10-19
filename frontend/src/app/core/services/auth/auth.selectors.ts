import { computed } from '@angular/core';

import { authUser, userPermissions } from './auth.state';

import { DEPARTMENT_SELECT, ROLE_SELECT, SUPER_ROLES } from '@/define';

export const isLoggedIn = computed(() => !!authUser());

export const userProfile = computed(() => authUser()?.userProfile);
export const userId = computed(() => authUser()?.id);
export const userFullname = computed(() => userProfile()?.fullname ?? '---');
export const userAvatar = computed(() => userProfile()?.avatar);
export const departmentListId = computed(() => userProfile()?.departmentList ?? []);
export const branchListId = computed(() => authUser()?.branchId ?? []);
export const branchActiveId = computed(() => userProfile()?.branchActive_id);

export const userRoleName = computed(() => authUser()?.userRole?.name);
export const userRoleDescription = computed(
  () => authUser()?.userRole?.description ?? 'Chưa phân quyền',
);

// Department
export const isDepartmentBOD = computed(() =>
  departmentListId().includes(DEPARTMENT_SELECT.boardOfDirectors),
);
export const isDepartmentDoctor = computed(() =>
  departmentListId().includes(DEPARTMENT_SELECT.doctor),
);
export const isDepartmentAssistant = computed(() =>
  departmentListId().includes(DEPARTMENT_SELECT.assistant),
);
export const isDepartmentTelesale = computed(() =>
  departmentListId().includes(DEPARTMENT_SELECT.telesale),
);
export const isDepartmentReceptionist = computed(() =>
  departmentListId().includes(DEPARTMENT_SELECT.receptionist),
);
export const isDepartmentCustomerCare = computed(() =>
  departmentListId().includes(DEPARTMENT_SELECT.customerCare),
);
export const isDepartmentConvert = computed(() =>
  departmentListId().includes(DEPARTMENT_SELECT.convert),
);
// Permission helpers
export const isDevelop = computed(() => userRoleName() === ROLE_SELECT.develop);
export const isAdmin = computed(() => userRoleName() === ROLE_SELECT.admin);
export const isSaleStaff = computed(() => departmentListId().includes(DEPARTMENT_SELECT.saleStaff));
export const isMarketing = computed(() => departmentListId().includes(DEPARTMENT_SELECT.marketing));

// Kiểm tra user có quyền trên branch cụ thể
export const hasBranchAccess = (branchId: number) =>
  computed(() => branchListId().includes(branchId));

export const hasDepartment = (dept: number) => computed(() => departmentListId().includes(dept));

/** Super user: role thuộc whitelist (mirror backend SuperRoles) */
export const isSuperUser = computed(() => {
  const role = authUser()?.userRole?.name ?? '';
  return (SUPER_ROLES as readonly string[]).includes(role);
});

/**
 * Quyền ký bệnh án — chỉ bác sĩ (department=doctor) hoặc super user mới được
 * ký số / chuyển record_status sang 'completed'. Tuân theo Thông tư 46/2018-BYT
 * và Nghị định 130/2018-NĐ-CP về chữ ký điện tử trong hồ sơ bệnh án.
 *
 * BE cũng phải enforce check này — đây là defense-in-depth lớp FE.
 */
export const canSignMedicalRecord = computed(() => {
  if (isSuperUser()) return true;
  return departmentListId().includes(DEPARTMENT_SELECT.doctor);
});

/** O(1) check. Super → true bất chấp set. */
export const hasPermission = (name: string): boolean => {
  if (isSuperUser()) return true;
  return userPermissions().has(name);
};

export const hasAnyPermission = (...names: string[]): boolean => {
  if (isSuperUser()) return true;
  const set = userPermissions();
  return names.some((n) => set.has(n));
};

export const hasAllPermissions = (...names: string[]): boolean => {
  if (isSuperUser()) return true;
  const set = userPermissions();
  return names.every((n) => set.has(n));
};
