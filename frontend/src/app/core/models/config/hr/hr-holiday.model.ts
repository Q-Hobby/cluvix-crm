import { Identifiable } from '@/core/interface/identifiable';
import { UserProfileModel } from '@/core/models/user-profile.model';

/**
 * Quản lý ngày lễ / nghỉ lễ trong hệ thống nhân sự.
 *
 * === LUỒNG HOẠT ĐỘNG ===
 *
 * 1. TẠO: FE gửi name, start_date, end_date, year, is_recurring → status = 1
 * 2. SỬA: cập nhật tất cả fields
 * 3. XÓA: cho phép xóa tự do
 *
 * - is_recurring: 0=Không lặp (ngày lễ đặc biệt), 1=Lặp lại hàng năm (1/1, 30/4, 2/9...)
 * - Dùng trong chấm công để xác định ngày nghỉ lễ
 */
export interface HrHolidayModel extends Identifiable {
  id?: number;
  name?: string;
  start_date?: number;
  end_date?: number;
  year?: number;
  is_recurring?: number;
  desc?: string;
  status?: number;
  created_by?: number;
  updated_by?: number;
  created_at?: number;
  updated_at?: number;
  userCreated?: UserProfileModel;
  userUpdated?: UserProfileModel;
}
