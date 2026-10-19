import { Identifiable } from '@/core/interface/identifiable';
import { UserProfileModel } from '@/core/models/user-profile.model';

/**
 * Cấu hình các thành phần lương trong hệ thống nhân sự.
 *
 * === LUỒNG HOẠT ĐỘNG ===
 *
 * 1. TẠO: Admin tạo thành phần lương mới với code duy nhất → status = 1
 * 2. SỬA: cập nhật tất cả fields, ảnh hưởng từ kỳ lương tiếp theo
 * 3. XÓA: cho phép xóa nếu chưa sử dụng trong bảng lương
 *
 * - type: 1=Thu nhập (cộng vào), 2=Khấu trừ (trừ đi)
 * - calc_type: 1=Cố định, 2=Theo % lương cơ bản, 3=Theo ngày công
 * - is_taxable: 0=Không chịu thuế, 1=Chịu thuế TNCN
 * - is_insurance: 0=Không tính BHXH, 1=Tính vào BHXH
 * - exempt_cap_type: null=không hạn mức, 1=Ăn ca (trần miễn thuế 1,2tr/tháng từ kỳ 07/2026 — NĐ 253/2026)
 */
export interface HrPayrollConfigModel extends Identifiable {
  id?: number;
  name?: string;
  code?: string;
  type?: number;
  calc_type?: number;
  default_amount?: number;
  percentage?: number;
  is_taxable?: number;
  is_insurance?: number;
  exempt_cap_type?: number | null;
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
