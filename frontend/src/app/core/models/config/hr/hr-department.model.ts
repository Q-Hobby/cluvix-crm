import { Identifiable } from '@/core/interface/identifiable';
import { UserProfileModel } from '@/core/models/user-profile.model';

/**
 * Quản lý phòng ban trong hệ thống nhân sự.
 *
 * === LUỒNG HOẠT ĐỘNG ===
 *
 * 1. TẠO: FE gửi name + desc → status = 1 (Hiển thị), SortOrder tự gán = id
 * 2. SỬA: Cập nhật name, desc, status. SortOrder hỗ trợ inline edit trên table
 * 3. XÓA: Chỉ phòng ban do người dùng tự tạo (id > 14). Phòng ban mặc định không được xóa
 *
 * Liên kết với user_profile qua departmentList (JSON array chứa department IDs)
 */
export interface HrDepartmentModel extends Identifiable {
  id?: number;
  name?: string;
  desc?: string;
  head_user_id?: number | null;
  status?: number;
  SortOrder?: number;
  created_by?: number;
  updated_by?: number;
  created_at?: number;
  updated_at?: number;
  userCreated?: UserProfileModel;
  userUpdated?: UserProfileModel;
}
