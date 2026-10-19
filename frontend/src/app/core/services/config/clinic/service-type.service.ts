import { inject, Service} from '@angular/core';
import { Observable } from 'rxjs';

import { ServiceTypeModel } from '@/core/models/config/clinic/service-type.model';
import { BaseApiService } from '@/share/services/base-api.service';
import { UserHelperService } from '@/indexeddb/config/components/user';
import { catchApiError } from '@/share/helpers/catch-api-error';

@Service()
export class ServiceTypeService extends BaseApiService<ServiceTypeModel> {
  protected override apiModule = 'config';
  protected override endpoint = 'service-type';
  protected override dateFields = [];
  protected override unixTimesTampFields = ['created_at'];
  private userHelper = inject(UserHelperService);

  override getInitialObject() {
    return {
      name: '',
      status: 1,
      emr_order_type: null,
    };
  }

  createServiceType(createParams: any, options: any = {}): Observable<any> {
    const params = this.beforeSave(createParams);
    return this.apiService.post<any>(`${this.path}`, params, options);
  }

  updateServiceType(id: number, updateParams: any, options: any = {}): Observable<any> {
    const params = this.beforeSave(updateParams);
    // PUT (không phải POST) — `resolveLoadingLabelKey` (loading-label.util.ts) suy label theo HTTP
    // method; POST luôn ra "Đang tạo mới" kể cả khi đang sửa. BE đã đăng ký PUT cùng handler
    // `UpdateServiceType` (route.go) nên đổi verb không cần đổi gì phía BE.
    return this.apiService.put<any>(`${this.path}/${id}`, params, options);
  }

  exportServiceType(params: any = {}): Observable<string> {
    return this.apiService.get(`${this.path}?_export=1`, params);
  }

  override importData(file: File): Observable<any> {
    const formData = new FormData();
    formData.append('file', file, file.name);
    return this.apiService
      .post(`${this.path}/import-from-excel`, formData)
      .pipe(catchApiError());
  }

  addLabel(item: ServiceTypeModel) {
    return { ...item };
  }

  async addLabels(items: ServiceTypeModel[]) {
    if (!Array.isArray(items)) return [];
    await this.ensureCacheLoaded();
    return items.map((item) => this.addLabel(item));
  }

  async ensureCacheLoaded() {
    await Promise.all([
      this.userHelper.ensureUsersLoaded(),
    ]);
  }
}
