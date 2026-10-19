/**
 * DTO cho các endpoint `/hr/dashboard/*`. BE chỉ trả dữ liệu động (số/list); field
 * trình bày (icon/màu/label) do FE tự map theo key — xem các component dashboard.
 */

/** Kỳ thống kê — chỉ áp dụng cho personnel-kpi + attendance-summary. */
export type HrDashboardRange = 'today' | 'week' | 'month';

/** `GET /hr/dashboard/personnel-kpi` */
export interface HrPersonnelKpi {
  active: number;
  new_in_period: number;
  left_in_period: number;
}

/** `GET /hr/dashboard/attendance-summary` */
export interface HrAttendanceSummary {
  attendance_rate: number;
  checked_in: number;
  active_total: number;
  late_count: number;
  early_count: number;
}

/** 1 dòng trong `GET /hr/dashboard/leave-today` — type: 2=Nghỉ phép, 4=Công tác. */
export interface HrLeaveTodayItem {
  user_id: number;
  fullname: string;
  code: string;
  type: number;
}

/** `GET /hr/dashboard/leave-today` — không nhận range, luôn là hôm nay. */
export interface HrLeaveToday {
  items: HrLeaveTodayItem[];
  total: number;
}

/** 1 dòng trong `GET /hr/dashboard/payroll-summary` — status: 1=Nháp,2=Chờ duyệt,3=Đã duyệt,4=Đã chi. */
export interface HrPayrollSummaryItem {
  status: number;
  count: number;
  /** decimal.Decimal từ BE — serialize thành string. */
  total: string;
}

/** `GET /hr/dashboard/payroll-summary` — không nhận range, luôn tháng hiện tại. */
export interface HrPayrollSummary {
  month: number;
  items: HrPayrollSummaryItem[];
  grand_total: string;
}

/** 1 dòng trong `GET /hr/dashboard/contract-expiring` — reason: contract_end | probation_end. */
export interface HrContractExpiringItem {
  contract_id: number;
  user_id: number;
  fullname: string;
  reason: 'contract_end' | 'probation_end';
  /** "YYYY-MM-DD" */
  expire_date: string;
}

/** `GET /hr/dashboard/contract-expiring?days=` — không nhận range, luôn từ hôm nay. */
export interface HrContractExpiring {
  items: HrContractExpiringItem[];
  total: number;
}

/** 1 dòng trong `GET /hr/dashboard/certificate-expiring` — type: 1=CCHN KCB, 2=CC chuyên khoa, 3=Khác. */
export interface HrCertificateExpiringItem {
  certificate_id: number;
  user_id: number;
  fullname: string;
  code: string;
  type: number;
  /** Tên chứng chỉ tự do khi type=Khác. */
  type_name: string | null;
  cert_no: string | null;
  /** "YYYY-MM-DD" — luôn có giá trị (BE loại expiry_date NULL = vô thời hạn). */
  expiry_date: string;
  days_left: number;
}

/** `GET /hr/dashboard/certificate-expiring?days=30|60|90` — không nhận range, luôn từ hôm nay. */
export interface HrCertificateExpiring {
  items: HrCertificateExpiringItem[];
  total: number;
}

/** `GET /hr/dashboard/allowance-bonus-summary` — không nhận range, luôn tháng hiện tại. */
export interface HrAllowanceBonusSummary {
  month: number;
  /** decimal.Decimal từ BE — serialize thành string. */
  allowance_total: string;
  bonus_total: string;
  penalty_total: string;
}
