import { Identifiable } from '@/core/interface/identifiable';
import { UserProfileModel } from '@/core/models/user-profile.model';
import { HrShiftModel } from '@/core/models/config/hr/hr-shift.model';

/**
 * Lịch làm việc / phân ca cho nhân viên.
 *
 * === LUỒNG HOẠT ĐỘNG ===
 *
 * 1. TẠO: Quản lý phân ca cho nhân viên theo ngày (đơn lẻ hoặc hàng loạt)
 * 2. SỬA: đổi ca, đổi type
 * 3. XÓA: cho phép xóa tự do
 *
 * - type: 1=Làm việc, 2=Nghỉ phép, 3=Nghỉ lễ, 4=Công tác
 * - shift_id: bắt buộc khi type=1, null khi nghỉ
 * - Unique: 1 nhân viên chỉ có 1 record / ngày
 */
export interface HrScheduleModel extends Identifiable {
  id?: number;
  user_id?: number;
  shift_id?: number;
  work_date?: number;
  type?: number;
  note?: string;
  status?: number;
  created_by?: number;
  updated_by?: number;
  created_at?: number;
  updated_at?: number;
  shift?: HrShiftModel;
  userProfile?: UserProfileModel;
  userCreated?: UserProfileModel;
  userUpdated?: UserProfileModel;
}

export interface BulkAssignPayload {
  user_ids: number[];
  shift_id: number | null;
  date_from: number;
  date_to: number;
  type: number;
  note?: string;
}

export interface CalendarResponse {
  schedules: Record<number, HrScheduleModel[]>;
  holidays: any[];
}
