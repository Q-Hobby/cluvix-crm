import {inject, Service} from '@angular/core';
import {Observable} from 'rxjs';

import {BaseApiService} from '@/share/services/base-api.service';
import {BranchHelperService} from '@/indexeddb/config/components/branch';
import {ServicesHelperService} from '@/indexeddb/config/components/services';
import {UserHelperService} from '@/indexeddb/config/components/user';

/**
 * Báo cáo theo nhân viên (BE `crm/report/staff`): doanh số + hoa hồng tư vấn viên.
 * Mỗi báo cáo có màn chi tiết (list) và màn tổng hợp (`/summary`, GROUP BY tư vấn viên).
 * `_export=1` ⇒ BE trả URL file Excel.
 */
@Service()
export class ReportStaffCrmService extends BaseApiService<any> {
  protected override apiModule = 'crm';
  protected override endpoint = 'report/staff';

  private branchHelper = inject(BranchHelperService);
  private servicesHelper = inject(ServicesHelperService);
  private userHelper = inject(UserHelperService);

  // --- Doanh số tư vấn viên (order_revenue_telesale) ---

  revenueTelesale(params: any = {}): Observable<any> {
    return this.apiService.get(`${this.path}/revenue-telesale`, params);
  }

  exportRevenueTelesale(params: any = {}): Observable<string> {
    return this.apiService.get(`${this.path}/revenue-telesale?_export=1`, params);
  }

  revenueTelesaleSummary(params: any = {}): Observable<any> {
    return this.apiService.get(`${this.path}/revenue-telesale/summary`, params);
  }

  exportRevenueTelesaleSummary(params: any = {}): Observable<string> {
    return this.apiService.get(`${this.path}/revenue-telesale/summary?_export=1`, params);
  }

  // --- Hoa hồng tư vấn viên (order_commission_telesale) ---

  commissionTelesale(params: any = {}): Observable<any> {
    return this.apiService.get(`${this.path}/commission-telesale`, params);
  }

  exportCommissionTelesale(params: any = {}): Observable<string> {
    return this.apiService.get(`${this.path}/commission-telesale?_export=1`, params);
  }

  commissionTelesaleSummary(params: any = {}): Observable<any> {
    return this.apiService.get(`${this.path}/commission-telesale/summary`, params);
  }

  exportCommissionTelesaleSummary(params: any = {}): Observable<string> {
    return this.apiService.get(`${this.path}/commission-telesale/summary?_export=1`, params);
  }

  /**
   * Gắn nhãn cho dòng màn chi tiết: tên khách, chi nhánh, dịch vụ, tư vấn viên (từ cache IndexedDB —
   * BE cố ý không attach user_profile ở màn chi tiết).
   */
  async addLabelsTelesale(items: any[]): Promise<any[]> {
    if (!Array.isArray(items)) return [];
    await Promise.all([
      this.branchHelper.ensureBranchesLoaded(),
      this.userHelper.ensureUsersLoaded(),
      this.servicesHelper.ensureServicesLoaded(),
    ]);
    return items.map((item) => {
      const withDates = this.addFormattedTimestampFields(item, ['confirm_time']);
      return {
        ...withDates,
        customerDisplayName: `${item?.customer?.fullname || ''}${item?.customer?.nickname ? ' - ' + item.customer.nickname : ''}`,
        branchLabel: this.branchHelper.getBranchName(item?.branch_id),
        telesale: this.userHelper.getUserInfo(item?.revenue_id),
        serviceInfo: this.servicesHelper.getServiceInfo(item?.service_id),
      };
    });
  }
}
