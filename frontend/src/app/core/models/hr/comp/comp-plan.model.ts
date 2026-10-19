import { Identifiable } from '@/core/interface/identifiable';

/**
 * Công thức lương/hoa hồng/KPI (comp_plan) — salary-commission-engine.
 *
 * 1 plan gồm nhiều component (fixed/tiered_commission/passthrough_commission/point_bonus/
 * kpi_bonus); tiered_commission có thêm `tiers[]`, kpi_bonus có thêm `criteria[]`. GET :id
 * trả nested đầy đủ (`PlanDetail` — `backend/internal/modules/webapp/hr/comp_plan/service/plan.go`);
 * POST/PUT gửi lại toàn bộ `components[]` (replace-all, không patch từng phần tử).
 */
export interface CompPlanModel extends Identifiable {
  id?: number;
  branch_id?: number;
  name?: string;
  status?: number; // 1=Hoạt động, 0=Ẩn
  note?: string;
  version?: number;
  created_at?: number;
  updated_at?: number;
  created_by?: number;
  updated_by?: number;
  components?: CompPlanComponentModel[];
}

/**
 * 1 thành phần công thức — type tra `COMP_COMPONENT_TYPE` (`@/define`). `params` là shape WIRE
 * (BE GET trả nested, POST/PUT gửi nested — xem `getPayload()`/`ngOnInit()` trong `comp-plan-form.ts`).
 * 8 field flat bên dưới (`amount`.."missing_manual") là bản FLATTEN CỦA `params` CHỈ DÙNG CHO FORM
 * (Signal Forms bind trực tiếp field phẳng qua `[formField]`, không bind nested `params.x` — mirror
 * đúng cách `buildComponentGroup()`/`getPayload()` bản Reactive Forms gốc đã flatten/unflatten thủ
 * công). Load xong — `ngOnInit()` flatten `params` → field phẳng 1 lần; submit — `getPayload()` gộp
 * ngược field phẳng → `params` theo đúng `type`.
 */
export interface CompPlanComponentModel {
  id?: number;
  type: number;
  role?: number | null;
  enabled?: number;
  sort_order?: number;
  params?: CompComponentParams;
  tiers?: CompCommissionTierModel[];
  criteria?: CompKpiCriteriaModel[];
  // --- FE-only, flatten của `params` (xem comment trên) ---
  amount?: number | null;
  unit_price?: number | null;
  calc_mode?: string | null;
  scope?: string | null;
  group_alloc?: string | null;
  target_bonus?: number | null;
  over_cap_percent?: number | null;
  missing_manual?: string | null;
}

/**
 * params JSON theo type — mirror `comp_engine/service/params.go ComponentParams`.
 * fixed: amount | point_bonus: unit_price | tiered_commission: calc_mode/scope/group_alloc |
 * kpi_bonus: target_bonus/over_cap_percent/missing_manual | passthrough: không cần params.
 */
export interface CompComponentParams {
  amount?: number | null;
  unit_price?: number | null;
  calc_mode?: string | null;
  scope?: string | null;
  group_alloc?: string | null;
  target_bonus?: number | null;
  over_cap_percent?: number | null;
  missing_manual?: string | null;
}

/** 1 bậc hoa hồng luỹ tiến (type=tiered_commission) — to_amount=null ở bậc cuối = vô hạn. */
export interface CompCommissionTierModel {
  id?: number;
  from_amount: number;
  to_amount: number | null;
  rate: number;
  sort_order?: number;
}

/** 1 tiêu chí KPI (type=kpi_bonus) — source tra `COMP_KPI_SOURCE` (`@/define`). */
export interface CompKpiCriteriaModel {
  id?: number;
  name: string;
  weight: number;
  target?: number | null;
  source?: number | null;
  source_role?: number | null;
  over_cap_percent?: number | null;
  sort_order?: number;
}
