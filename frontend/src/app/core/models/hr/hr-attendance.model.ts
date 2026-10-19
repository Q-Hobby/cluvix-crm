import { Identifiable } from '@/core/interface/identifiable';
import { UserProfileModel } from '@/core/models/user-profile.model';

/**
 * Bảng chấm công - dữ liệu import từ Excel.
 *
 * === LUỒNG HOẠT ĐỘNG ===
 *
 * 1. IMPORT: Quản lý upload file Excel chấm công → BE parse & lưu
 * 2. XEM: Bảng tổng hợp theo tháng (hàng = nhân viên, cột = ngày)
 * 3. CHI TIẾT: Xem chi tiết chấm công 1 nhân viên theo khoảng thời gian
 *
 * - working_day: unix timestamp (ngày làm việc)
 * - workday: công tính theo ngày (full = 1)
 * - hourday: công tính theo giờ
 * - Liên kết với UserProfile qua employee_code → user_profile.code
 */
export interface HrAttendanceModel extends Identifiable {
  id?: number;
  employee_code?: string;
  department_id?: number;
  employee_position?: number;
  working_day?: number;
  day_of_week?: string;
  checkin?: string;
  checkout?: string;
  workday?: number;
  hourday?: number;
  hourdayplus?: number;
  workdayplus?: number;
  late_for_work?: number;
  return_early?: number;
  ot_1?: number;
  ot_2?: number;
  ot_3?: number;
  name_ot?: string;
  total_hours?: number;
  time_import?: number;
  import_by?: number;
  checkin_method?: string;
  checkout_method?: string;
  employee?: UserProfileModel;
}

/**
 * Response từ API index: UserProfile + attendance[]
 */
export interface AttendanceUserRow extends UserProfileModel {
  attendance?: HrAttendanceModel[];
  totalWorkDay?: number;
}
