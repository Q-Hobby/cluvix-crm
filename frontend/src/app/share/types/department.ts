// types/department.ts
import { DEPARTMENT_SELECT } from '@/define';

// Type id phòng ban (tự suy ra từ DEPARTMENT, nếu sau này đổi qua string/enums vẫn đúng)
export type DepartmentId = (typeof DEPARTMENT_SELECT)[keyof typeof DEPARTMENT_SELECT];
