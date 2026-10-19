import { inject, Service} from '@angular/core';
import { Observable } from 'rxjs';
import { map } from 'rxjs/operators';

import { BankAccountModel } from '@/core/models/config/accountant/bank-account.model';
import { BaseApiService } from '@/share/services/base-api.service';
import { ApiResponseOneModel } from '@/core/interface/ApiResponseOneModel';
import { ListApiResponse } from '@/share/models/list-api-response';
import { toFormData } from '@/share/services/utils.service';
import { catchApiError } from '@/share/helpers/catch-api-error';

@Service()
export class BankAccountService extends BaseApiService<BankAccountModel> {
  protected override apiModule = 'config';
  protected override endpoint = 'bank-account';
  protected override unixTimesTampFields = ['created_at', 'updated_at'];

  override getInitialObject() {
    return {
      bank_code: '',
      account_no: '',
      account_name: '',
      description: '',
      status: 1,
      is_default: 0,
    };
  }

  override list(params: any = {}): Observable<ListApiResponse<BankAccountModel>> {
    // Đảm bảo page >= 1
    params.page = params.page ? (params.page < 1 ? 1 : params.page) : 1;

    return this.apiService.get<ListApiResponse<BankAccountModel>>(`${this.path}`, params).pipe(
      map((listApiResponse: ListApiResponse<BankAccountModel>) => {
        const cloned = JSON.parse(JSON.stringify(listApiResponse));
        if (cloned.items) {
          cloned.items = cloned.items.map((model: BankAccountModel) => {
            return this.afterLoad(model);
          });
        }
        return cloned;
      }),
    );
  }

  // Tạo mới bank account
  createBankAccount(
    createParams: any = {},
    options: any = {},
    isToFormData: any = false,
  ): Observable<any> {
    const params = this.beforeSave(createParams);
    return this.apiService.post(
      `${this.path}`,
      isToFormData ? toFormData(params) : params,
      options,
    );
  }

  // Cập nhật bank account
  updateBankAccount(
    id: number,
    updateParams: any = {},
    options: any = {},
    isToFormData: any = false,
  ): Observable<ApiResponseOneModel<BankAccountModel>> {
    const params = this.beforeSave(updateParams);
    return this.apiService
      .put<
        ApiResponseOneModel<BankAccountModel>
      >(`${this.path}/${id}`, isToFormData ? toFormData(params) : params, options)
      .pipe(
        map((res: any) => {
          const cloned = JSON.parse(JSON.stringify(res));
          if (cloned.data) {
            cloned.data = this.afterLoad(cloned.data);
          }
          return cloned;
        }),
      );
  }

  // Thêm labels cho một item
  addLabel(item: BankAccountModel) {
    return { ...item };
  }

  // Thêm labels cho danh sách
  async addLabels(items: BankAccountModel[]): Promise<BankAccountModel[]> {
    if (!Array.isArray(items) || items.length === 0) return items;
    return items.map((item) => this.addLabel(item));
  }

  // Xử lý thay đổi trạng thái mặc định
  handleDefaultChange(params: { id: number }): Observable<any> {
    return this.apiService.post(`${this.path}/set-default`, params);
  }

  // Export dữ liệu
  exportBankAccount(options: any = {}): Observable<any> {
    return this.apiService.get(`${this.path}?_export=1`, options);
  }

  // Import dữ liệu
  override importData(file: File) {
    const formData = new FormData();
    formData.append('file', file, file.name);

    return this.apiService
      .post(`${this.path}/import-bank-account-from-excel`, formData)
      .pipe(catchApiError());
  }
}
