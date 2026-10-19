import { Identifiable } from '@/core/interface/identifiable';
import { UserProfileModel } from '@/core/models/user-profile.model';

/**
 * Thưởng / Phạt hàng tháng cho nhân viên.
 *
 * === LUỒNG HOẠT ĐỘNG ===
 *
 * 1. TẠO: HR chọn nhân viên + loại + số tiền + tháng + lý do → status = 1
 * 2. SỬA: cập nhật số tiền, lý do
 * 3. XÓA: cho phép xóa tự do khi chưa tính lương
 *
 * - type: 1=Thưởng (cộng vào lương), 2=Phạt (trừ khỏi lương)
 * - month: format YYYYMM (VD: 202603)
 */
export interface HrBonusModel extends Identifiable {
  id?: number;
  user_id?: number;
  type?: number;
  amount?: number;
  month?: number;
  reason?: string;
  note?: string;
  status?: number;
  version?: number;
  created_by?: number;
  updated_by?: number;
  created_at?: number;
  updated_at?: number;
  userProfile?: UserProfileModel;
}
