import { Service} from '@angular/core';

import { BranchModel } from '@/core/models/config/system/branch.model';
import { BaseApiService } from '@/share/services/base-api.service';
import { Observable } from 'rxjs';

@Service()
export class BranchService extends BaseApiService<BranchModel> {
  protected override apiModule = 'config';
  protected override endpoint = 'branch';
  protected override dateFields = [];

  // Signal Forms cần đủ key với giá trị non-`undefined` để bind field (gotcha #8/#1 `angular22.md`) —
  // `name`/`code`/`hotline`/`email` bind `<input pInputText>` seed `''` (gotcha #9), `address` seed `null`.
  override getInitialObject() {
    return {
      name: '',
      code: '',
      address: null,
      hotline: '',
      email: '',
      status: 1,
      statusOpenAppointment: 1,
    };
  }

  showHideOpenAppointment(params: any): Observable<string> {
    return this.apiService.post(`${this.path}/status-open-appointment`, params, {});
  }

  /**
   * @param createParams
   * @param options
   */
  createBranch(createParams: any, options: any = {}): Observable<any> {
    const params = this.beforeSave(createParams);
    return this.apiService.post<any>(`${this.path}`, params, options);
  }

  /**
   * @param id
   * @param updateParams
   * @param options
   */
  updateBranch(id: number, updateParams: any, options: any = {}): Observable<any> {
    const params = this.beforeSave(updateParams);
    // PUT (không phải POST) — `resolveLoadingLabelKey` (loading-label.util.ts) suy label theo HTTP
    // method; POST luôn ra "Đang tạo mới" kể cả khi đang sửa. BE đã đăng ký PUT cùng handler
    // `UpdateBranch` (route.go) nên đổi verb không cần đổi gì phía BE.
    return this.apiService.put<any>(`${this.path}/${id}`, params, options);
  }

  addLabel(item: BranchModel) {
    return { ...item };
  }

  async addLabels(items: BranchModel[]) {
    if (!Array.isArray(items)) return [];
    return items.map((item) => this.addLabel(item));
  }
}
