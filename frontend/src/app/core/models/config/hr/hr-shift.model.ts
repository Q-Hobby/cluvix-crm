import { Identifiable } from '@/core/interface/identifiable';
import { UserProfileModel } from '@/core/models/user-profile.model';

/**
 * Cấu hình ca làm việc trong hệ thống chấm công.
 *
 * === LUỒNG HOẠT ĐỘNG ===
 *
 * 1. TẠO: Admin tạo ca mới với code duy nhất, giờ bắt đầu/kết thúc → status = 1
 * 2. SỬA: cập nhật tất cả fields, ảnh hưởng từ ngày tiếp theo
 * 3. XÓA: cho phép xóa nếu chưa gán trong lịch làm việc
 *
 * - is_overnight: 0=Ca bình thường, 1=Ca qua đêm (end_time < start_time)
 * - late_tolerance/early_tolerance: phút cho phép trễ/sớm trước khi tính vi phạm
 * - Dùng trong phân ca và đối chiếu chấm công
 */
export interface HrShiftModel extends Identifiable {
  id?: number;
  name?: string;
  code?: string;
  start_time?: string;
  end_time?: string;
  break_start?: string;
  break_end?: string;
  work_hours?: number;
  is_overnight?: number;
  late_tolerance?: number;
  early_tolerance?: number;
  color?: string;
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
