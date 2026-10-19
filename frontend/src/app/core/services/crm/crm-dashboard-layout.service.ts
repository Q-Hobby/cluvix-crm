import { HttpContext } from '@angular/common/http';
import { Service } from '@angular/core';
import { map, Observable } from 'rxjs';

import { BaseApiService } from '@/share/services/base-api.service';
import { SKIP_GLOBAL_403, SKIP_GLOBAL_404 } from '@/core/interceptors/response-handler.interceptor';

/**
 * Bố cục hiệu lực BE trả (`crm/dashboard_layout`): của user > mặc định thương hiệu > hệ thống (config null).
 * CRM: 1 DB = 1 thương hiệu ⇒ "mặc định thương hiệu" thay "mặc định công ty" của ERP.
 */
export interface DashboardLayoutView {
  config: unknown;
  source: 'user' | 'brand' | 'system';
  hasBrandDefault: boolean;
  updatedAt: number;
}

/** API bố cục Dashboard CRM tuỳ biến. */
@Service()
export class CrmDashboardLayoutService extends BaseApiService<any> {
  override apiModule = 'crm';
  override endpoint = 'crm-dashboard/layout';

  /**
   * Đọc bố cục KHÔNG được làm cả trang đổi hướng: lỗi (404 khi BE chưa có route, 403…) để store tự
   * rơi về bản tạm / mặc định hệ thống — dashboard vẫn dùng được. Interceptor mặc định đá 404 sang
   * /auth/notfound (sự cố 2026-10-19: BE dev chạy binary cũ ⇒ vào trang chủ thành notfound).
   */
  getLayout(): Observable<DashboardLayoutView> {
    const context = new HttpContext().set(SKIP_GLOBAL_404, true).set(SKIP_GLOBAL_403, true);
    return this.apiService.get<{ data: DashboardLayoutView }>(this.path, { context }).pipe(map((r) => r.data));
  }

  saveLayout(config: unknown): Observable<DashboardLayoutView> {
    return this.apiService.put<{ data: DashboardLayoutView }>(this.path, { config }).pipe(map((r) => r.data));
  }

  resetLayout(): Observable<DashboardLayoutView> {
    return this.apiService.delete<{ data: DashboardLayoutView }>(this.path).pipe(map((r) => r.data));
  }

  saveBrandDefault(config: unknown): Observable<DashboardLayoutView> {
    return this.apiService.put<{ data: DashboardLayoutView }>(`${this.path}/brand-default`, { config }).pipe(map((r) => r.data));
  }

  deleteBrandDefault(): Observable<DashboardLayoutView> {
    return this.apiService.delete<{ data: DashboardLayoutView }>(`${this.path}/brand-default`).pipe(map((r) => r.data));
  }
}
