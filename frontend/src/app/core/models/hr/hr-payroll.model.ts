import { Identifiable } from '@/core/interface/identifiable';
import { UserProfileModel } from '@/core/models/user-profile.model';

/**
 * Bảng lương tổng hợp hàng tháng cho nhân viên.
 *
 * === LUỒNG HOẠT ĐỘNG ===
 *
 * 1. TẠO: HR chọn tháng → Tính lương hàng loạt → status = 1 (Nháp)
 * 2. SỬA: chỉ khi status = 1
 * 3. GỬI DUYỆT: status 1→2
 * 4. DUYỆT: status 2→3
 * 5. CHI LƯƠNG: status 3→4
 * 6. XÓA: chỉ khi status = 1
 *
 * - status: 1=Nháp, 2=Chờ duyệt, 3=Đã duyệt, 4=Đã chi
 * - month: format YYYYMM
 */
export interface HrPayrollModel extends Identifiable {
  id?: number;
  user_id?: number;
  month?: number;
  base_salary?: number;
  standard_workdays?: number;
  actual_workdays?: number;
  ot_hours?: number;
  ot_amount?: number;
  total_allowance?: number;
  total_bonus?: number;
  total_penalty?: number;
  total_deduction?: number;
  /** Tổng tạm ứng trừ vào kỳ (story-11/12 hr-completion) — hiển thị, BE tự tính. */
  total_advance?: number;
  tax_income?: number;
  tax_amount?: number;
  net_salary?: number;
  note?: string;
  status?: number;
  created_by?: number;
  updated_by?: number;
  created_at?: number;
  updated_at?: number;
  userProfile?: UserProfileModel;
  details?: HrPayrollDetailModel[];
}

export interface HrPayrollDetailModel {
  id?: number;
  payroll_id?: number;
  payroll_config_id?: number;
  type?: number;
  amount?: number;
  note?: string;
  payrollConfig?: { id?: number; name?: string; code?: string; type?: number; calc_type?: number };
}
