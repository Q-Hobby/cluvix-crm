import { Identifiable } from '@/core/interface/identifiable';
import { UserProfileModel } from '@/core/models/user-profile.model';

/**
 * Cấu hình chính sách nghỉ phép trong hệ thống nhân sự.
 *
 * === LUỒNG HOẠT ĐỘNG ===
 *
 * 1. TẠO: Admin tạo loại phép mới với code duy nhất, số ngày/năm → status = 1
 * 2. SỬA: cập nhật tất cả fields, ảnh hưởng từ năm tiếp theo
 * 3. XÓA: cho phép xóa nếu chưa có đơn nghỉ phép nào sử dụng
 *
 * - type: 1=Có lương, 2=Không lương
 * - is_accumulative: 0=Không tích lũy, 1=Tích lũy theo thâm niên
 * - gender: null=Tất cả, 1=Nam, 2=Nữ (VD: thai sản chỉ áp dụng nữ)
 * - Tổng phép = days_per_year + (thâm niên × accumulative_days)
 */
export interface HrLeavePolicyModel extends Identifiable {
  id?: number;
  name?: string;
  code?: string;
  type?: number;
  days_per_year?: number;
  max_carry_over?: number;
  is_accumulative?: number;
  accumulative_days?: number;
  max_consecutive?: number;
  advance_days?: number;
  gender?: number;
  SortOrder?: number;
  desc?: string;
  status?: number;
  created_by?: number;
  updated_by?: number;
  created_at?: number;
  updated_at?: number;
  userCreated?: UserProfileModel;
  userUpdated?: UserProfileModel;
}
