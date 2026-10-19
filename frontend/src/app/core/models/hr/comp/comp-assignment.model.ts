import { Identifiable } from '@/core/interface/identifiable';

/**
 * Gán công thức lương/hoa hồng (comp_plan_assignment) cho phòng ban HOẶC cá nhân —
 * salary-commission-engine story-15. Đúng 1 trong `department_id`/`user_id` có giá trị (XOR,
 * BE validate `comp_assignment/handler/handler.go validateTarget`). `effective_to = null` =
 * còn hiệu lực. List trả kèm `plan_name`/`department_name`/`user_fullname` (LEFT JOIN, BE
 * `ListCompAssignment`); GET :id trả nested `plan` (`CompPlanAssignmentWithRel`).
 */
/** Radio chọn kiểu đối tượng gán — chỉ FE, không gửi lên BE (XOR department_id/user_id). */
export type CompAssignmentTargetType = 'department' | 'user';

export interface CompAssignmentModel extends Identifiable {
  id?: number;
  plan_id: number;
  // FE-only — quyết định department_id/user_id nào đang active, loại khỏi payload ở getPayload().
  // Field không tồn tại trên BE, phải khai trong model vì Signal Forms cần key thật để bind field
  // (rootField.x trả undefined nếu key vắng mặt, kể cả field không submit — xem angular22.md gotcha #1).
  target_type?: CompAssignmentTargetType | null;
  department_id?: number | null;
  user_id?: number | null;
  effective_from: string; // YYYY-MM-DD
  effective_to?: string | null; // YYYY-MM-DD | null = còn hiệu lực
  version?: number;
  created_at?: number;
  updated_at?: number;
  created_by?: number;
  updated_by?: number;
  // Chỉ có ở response list (LEFT JOIN) hoặc get (Preload) — không gửi lại khi POST/PUT.
  plan_name?: string;
  department_name?: string;
  user_fullname?: string;
  plan?: { id: number; name: string };
}
