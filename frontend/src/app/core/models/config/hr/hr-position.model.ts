import { Identifiable } from '@/core/interface/identifiable';
import { UserProfileModel } from '@/core/models/user-profile.model';

/**
 * Quản lý chức vụ nhân viên trong hệ thống nhân sự.
 *
 * === LUỒNG HOẠT ĐỘNG ===
 *
 * 1. TẠO: FE gửi name + desc → status = 1 (Hiển thị), SortOrder tự gán = id
 * 2. SỬA: Cập nhật name, desc, status. SortOrder hỗ trợ inline edit trên table
 * 3. XÓA: Cho phép xóa tất cả chức vụ
 *
 * Liên kết với hr_attendance qua employee_position (FK → hr_position.id)
 */
export interface HrPositionModel extends Identifiable {
  id?: number;
  name?: string;
  desc?: string;
  status?: number;
  SortOrder?: number;
  created_by?: number;
  updated_by?: number;
  created_at?: number;
  updated_at?: number;
  userCreated?: UserProfileModel;
  userUpdated?: UserProfileModel;
}
