import { Identifiable } from '@/core/interface/identifiable';

/** Trạng thái đơn nghỉ phép (BE hr_leave/dao: HrLeaveRequestStatus*). */
export const HR_LEAVE_REQUEST_STATUS = {
  DRAFT: 1,
  PENDING: 2,
  APPROVED: 3,
  REJECTED: 4,
  RETURNED: 5,
  CANCELLED: 6,
} as const;

/**
 * Đơn nghỉ phép (bảng `hr_leave_request`).
 * List BE JOIN sẵn user_profile + hr_leave_policy → có user_fullname/policy_name.
 * `days` là decimal-string từ BE → `Number()` khi render/sort.
 */
export interface HrLeaveRequestModel extends Identifiable {
  id?: number;
  user_id?: number;
  branch_id?: number | null;
  leave_policy_id?: number;
  /** "YYYY-MM-DD". */
  from_date?: string;
  /** "YYYY-MM-DD". */
  to_date?: string;
  /** Decimal-string từ BE (bước 0.5). */
  days?: string | number;
  /** Tường minh từ BE — KHÔNG suy từ days === 0.5 (fragile nếu đổi đơn vị phép sau này). */
  half_day?: boolean;
  reason?: string | null;
  /** 1=Nháp, 2=Chờ duyệt, 3=Duyệt, 4=Từ chối, 5=Trả lại, 6=Hủy. */
  status?: number;
  approved_by?: number | null;
  /** Unix (s). */
  approved_at?: number | null;
  /** Lý do trả lại / từ chối. */
  return_reason?: string | null;
  created_by?: number | null;
  updated_by?: number | null;
  created_at?: number;
  updated_at?: number;
  // JOIN từ list BE
  user_fullname?: string | null;
  user_code?: string | null;
  policy_name?: string | null;
  /** 1=Có lương (trừ phép), 2=Không lương. */
  policy_type?: number | null;
}

/**
 * Tồn phép theo user × loại phép × năm (bảng `hr_leave_balance`).
 * entitled/used/carried_over/adjusted là decimal-string từ BE → `Number()` khi render.
 */
export interface HrLeaveBalanceModel extends Identifiable {
  id?: number;
  user_id?: number;
  leave_policy_id?: number;
  year?: number;
  entitled?: string | number;
  used?: string | number;
  carried_over?: string | number;
  adjusted?: string | number;
  adjust_note?: string | null;
  created_at?: number;
  updated_at?: number;
  // JOIN từ list BE
  user_fullname?: string | null;
  user_code?: string | null;
  policy_name?: string | null;
  policy_type?: number | null;
}
