import { Identifiable } from '@/core/interface/identifiable';

/**
 * Kỳ tính lương/hoa hồng/KPI (comp_period) — salary-commission-engine story-09/10 (BE),
 * story-16 (FE). period = YYYYMM. status: 1=Nháp (chỉnh sửa/tính lại/chấm KPI được),
 * 2=Đã duyệt (bất biến — BE khoá, FE chỉ disable UI theo status này).
 */
export interface CompPeriodModel extends Identifiable {
  id?: number;
  period?: number;
  status?: number;
  calculated_at?: number | null;
  approved_by?: number | null;
  approved_at?: number | null;
  version?: number;
  created_at?: number;
  updated_at?: number;
  created_by?: number;
  updated_by?: number;
}

/** 1 cảnh báo trả về từ POST /calculate (handler.go periodWarning) — refund treo (engine)
 * hoặc user có doanh số nhưng không match plan nào (rủi ro #4 PRD). */
export interface CompPeriodCalculateWarning {
  type: string;
  osu_id?: number;
  user_id?: number;
  message?: string;
}

export interface CompPeriodCalculateResult {
  success: boolean;
  period: number;
  created: number;
  updated: number;
  total: number;
  warnings: CompPeriodCalculateWarning[];
}

/** 1 bậc hoa hồng luỹ tiến đã áp dụng — computed_breakdown.components[].tier_steps. */
export interface CompTierStepResult {
  from_amount: number;
  to_amount: number | null;
  rate: number;
  base: number;
  amount: number;
}

/** 1 tiêu chí KPI đã tính — computed_breakdown.components[].kpi_details[]. */
export interface CompKpiCriterionResult {
  criteria_id: number;
  name: string;
  weight: number;
  target: number | null;
  actual: number | null;
  ratio: number;
  contribution: number;
  missing: boolean;
}

/** 1 thành phần công thức đã tính cho 1 user/kỳ — mirror BE `ComponentResult`. */
export interface CompComponentResultRow {
  component_id: number;
  type: number;
  role?: number | null;
  amount: number;
  scope?: string;
  tier_mode?: string;
  tier_steps?: CompTierStepResult[];
  group_base?: boolean;
  kpi_score?: number | null;
  kpi_details?: CompKpiCriterionResult[];
}

/** Config 1 tiêu chí KPI tại thời điểm tính (input_snapshot.components[].criteria[]) —
 * DUY NHẤT nguồn xác định criteria có source=manual (chấm được) hay không, vì
 * computed_breakdown.kpi_details KHÔNG mang field `source`. */
export interface CompSnapshotCriteriaConfig {
  id: number;
  name: string;
  weight: number;
  target: number | null;
  source: number | null;
  source_role: number | null;
  over_cap_percent: number | null;
}

export interface CompSnapshotComponentConfig {
  id: number;
  type: number;
  role: number | null;
  params?: Record<string, any>;
  tiers?: any[];
  criteria?: CompSnapshotCriteriaConfig[];
}

/** input_snapshot (FR13 — ảnh bất biến đầu vào tại thời điểm tính). */
export interface CompPeriodInputSnapshot {
  period: number;
  plan_id: number | null;
  role_revenue?: Record<number, number>;
  passthrough?: Record<number, number>;
  kpi_actuals?: Record<number, number | null>;
  components: CompSnapshotComponentConfig[];
}

/** 1 dòng kết quả per-user của kỳ — GET /comp-period/:id/results. */
export interface CompPeriodResultModel extends Identifiable {
  id?: number;
  period_id?: number;
  user_id?: number;
  plan_id?: number | null;
  department_id?: number | null;
  total?: number;
  sum_commission?: number;
  sum_kpi?: number;
  sum_point?: number;
  matched?: number; // 1=khớp công thức, 0=không match plan nào (cảnh báo)
  input_snapshot?: CompPeriodInputSnapshot;
  computed_breakdown?: CompComponentResultRow[];
  version?: number;
  user_fullname?: string;
  department_name?: string;
}
