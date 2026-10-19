import { inject, Service } from '@angular/core';
import { HttpParams } from '@angular/common/http';
import { Observable } from 'rxjs';
import { map } from 'rxjs/operators';
import { ApiService } from '@/share/services/api.service';
import {
  HrAllowanceBonusSummary,
  HrAttendanceSummary,
  HrCertificateExpiring,
  HrContractExpiring,
  HrDashboardRange,
  HrLeaveToday,
  HrPayrollSummary,
  HrPersonnelKpi,
} from '@/core/models/hr/hr-dashboard.model';

/**
 * Service cho 6 endpoint dashboard HR `/hr/dashboard/*`. Mọi response bọc `{ data: ... }`
 * → unwrap về DTO. Chỉ `personnelKpi`/`attendanceSummary` nhận `range` — 4 endpoint còn
 * lại luôn "hôm nay"/"tháng hiện tại" theo BE, không nhận param.
 */
@Service()
export class HrDashboardService {
  private api = inject(ApiService);
  private readonly base = '/hr/dashboard';

  private unwrap<T>(): (res: any) => T {
    return (res: any) => (res?.data ?? res) as T;
  }

  personnelKpi(range: HrDashboardRange): Observable<HrPersonnelKpi> {
    return this.api
      .get<any>(`${this.base}/personnel-kpi`, { params: new HttpParams().set('range', range) })
      .pipe(map(this.unwrap<HrPersonnelKpi>()));
  }

  attendanceSummary(range: HrDashboardRange): Observable<HrAttendanceSummary> {
    return this.api
      .get<any>(`${this.base}/attendance-summary`, { params: new HttpParams().set('range', range) })
      .pipe(map(this.unwrap<HrAttendanceSummary>()));
  }

  leaveToday(): Observable<HrLeaveToday> {
    return this.api.get<any>(`${this.base}/leave-today`).pipe(map(this.unwrap<HrLeaveToday>()));
  }

  payrollSummary(): Observable<HrPayrollSummary> {
    return this.api.get<any>(`${this.base}/payroll-summary`).pipe(map(this.unwrap<HrPayrollSummary>()));
  }

  contractExpiring(days = 30): Observable<HrContractExpiring> {
    return this.api
      .get<any>(`${this.base}/contract-expiring`, { params: new HttpParams().set('days', String(days)) })
      .pipe(map(this.unwrap<HrContractExpiring>()));
  }

  certificateExpiring(days = 60): Observable<HrCertificateExpiring> {
    return this.api
      .get<any>(`${this.base}/certificate-expiring`, { params: new HttpParams().set('days', String(days)) })
      .pipe(map(this.unwrap<HrCertificateExpiring>()));
  }

  allowanceBonusSummary(): Observable<HrAllowanceBonusSummary> {
    return this.api
      .get<any>(`${this.base}/allowance-bonus-summary`)
      .pipe(map(this.unwrap<HrAllowanceBonusSummary>()));
  }
}
