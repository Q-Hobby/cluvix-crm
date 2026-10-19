import { Identifiable } from '@/core/interface/identifiable';
import { UserProfileModel } from '@/core/models/user-profile.model';

/** Trạng thái đơn tạm ứng lương (bảng `hr_salary_advance`). */
export const HR_SALARY_ADVANCE_STATUS = {
  /** Nháp — sửa/xóa được. */
  DRAFT: 1,
  /** Chờ duyệt. */
  PENDING: 2,
  /** Đã duyệt — chờ trừ vào kỳ lương `month` (story-12). */
  APPROVED: 3,
  /** Từ chối — sửa rồi gửi duyệt lại được. */
  REJECTED: 4,
  /** Đã trừ vào bảng lương (`deducted_payroll_id`). */
  DEDUCTED: 5,
} as const;

/**
 * Đơn tạm ứng lương — admin tạo thay nhân viên, duyệt 1 cấp.
 * Luồng: 1 Nháp → 2 Chờ duyệt → 3 Duyệt → 5 Đã trừ (BE set khi tính lương);
 * 2 → 4 Từ chối → 2 (sửa rồi gửi lại).
 */
export interface HrSalaryAdvanceModel extends Identifiable {
  id?: number;
  user_id?: number;
  branch_id?: number | null;
  /** Kỳ trừ lương YYYYMM. */
  month?: number;
  /** BE trả decimal dạng string — dùng Number() khi hiển thị/binding. */
  amount?: number | string;
  reason?: string | null;
  status?: number;
  approved_by?: number | null;
  /** Unix seconds. */
  approved_at?: number | null;
  /** FK hr_payroll.id khi status=5 (story-12). */
  deducted_payroll_id?: number | null;
  created_by?: number | null;
  updated_by?: number | null;
  created_at?: number;
  updated_at?: number;
  /** JOIN user_profile ở list BE. */
  user_fullname?: string;
  user_code?: string;
  userProfile?: UserProfileModel;
}
