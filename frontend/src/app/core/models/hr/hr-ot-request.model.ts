import { Identifiable } from '@/core/interface/identifiable';

/** Trạng thái đơn tăng ca (BE hr_ot_request/dao: HrOtRequestStatus*, mirror hr_leave_request). */
export const HR_OT_REQUEST_STATUS = {
  DRAFT: 1,
  PENDING: 2,
  APPROVED: 3,
  REJECTED: 4,
  RETURNED: 5,
  CANCELLED: 6,
} as const;

/**
 * Đơn xin tăng ca (bảng `hr_ot_request`) — story-04 hr-ot-request.
 * List BE JOIN sẵn `user_profile` → `userProfile{fullname,code}`.
 * `hours` là decimal-string từ BE (bước 0.5) → `Number()` khi render/sort.
 */
export interface HrOtRequestModel extends Identifiable {
  id?: number;
  user_id?: number;
  branch_id?: number | null;
  /** "YYYY-MM-DD". */
  work_date?: string;
  /** Decimal-string từ BE (bước 0.5, min 0.5, max 16). */
  hours?: string | number;
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
  version?: number;
  created_at?: number;
  updated_at?: number;
  /** LEFT JOIN user_profile trong ListOtRequest — nested ref (KHÔNG flatten), theo contract cuối. */
  userProfile?: { fullname?: string | null; code?: string | null } | null;
  /**
   * approval-workflow-engine story-13: id instance mới nhất (bất kỳ status) của phiếu — nullable,
   * `null`/`undefined` = phiếu chưa từng đi qua engine (handler.go ListOtRequest correlated subquery
   * + GetOtRequest latestInstanceID). Có giá trị → FE load timeline (ApprovalInstanceService) để quyết
   * định ẩn nút duyệt legacy (CHỈ khi instance đang `running`) và hiện action qua engine.
   */
  approval_instance_id?: number | null;
}
