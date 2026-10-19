import { Identifiable } from '@/core/interface/identifiable';
import { UserProfileModel } from '@/core/models/user-profile.model';
import { HrPayrollConfigModel } from '@/core/models/config/hr/hr-payroll-config.model';

/**
 * Phụ cấp hàng tháng cho nhân viên.
 *
 * === LUỒNG HOẠT ĐỘNG ===
 *
 * 1. TẠO: HR chọn nhân viên + loại phụ cấp + tháng + số tiền → status = 1
 * 2. SỬA: cập nhật số tiền, ghi chú
 * 3. XÓA: cho phép xóa tự do khi chưa tính lương
 *
 * - Unique: 1 nhân viên + 1 loại phụ cấp + 1 tháng = 1 record
 * - month: format YYYYMM (VD: 202603)
 */
export interface HrAllowanceModel extends Identifiable {
  id?: number;
  user_id?: number;
  payroll_config_id?: number;
  amount?: number;
  month?: number;
  note?: string;
  status?: number;
  version?: number;
  created_by?: number;
  updated_by?: number;
  created_at?: number;
  updated_at?: number;
  userProfile?: UserProfileModel;
  payrollConfig?: HrPayrollConfigModel;
}
