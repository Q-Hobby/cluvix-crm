import { inject, Service} from '@angular/core';
import { Observable } from 'rxjs';
import { map } from 'rxjs/operators';

import { ApiResponseOneModel } from '@/core/interface/ApiResponseOneModel';
import { ChangePasswordModel } from '@/core/models/change-password.model';
import { UserModel } from '@/core/models/hr/user.model';
import { catchApiError } from '@/share/helpers/catch-api-error';
import { LoginResponse } from '@/share/models/login-response';
import { BaseApiService } from '@/share/services/base-api.service';
import { UserHelperService } from '@/indexeddb/config/components/user';
import { BranchHelperService } from '@/indexeddb/config/components/branch';
import { DepartmentHelperService } from '@/indexeddb/config/components/hr-department';
import { getGenderKey, getGenderLabel, getUserStatusLabel, USER_STATUS } from '@/define';

@Service()
export class UserService extends BaseApiService<UserModel> {
  public override unixTimesTampFields = ['created_at', 'updated_at'];
  protected override apiModule = 'hr';
  protected override endpoint = 'user';
  protected override dateFields = ['birthday'];
  private userHelper = inject(UserHelperService);
  private branchHelper = inject(BranchHelperService);
  private departmentHelperService = inject(DepartmentHelperService);

  override getInitialObject(): Partial<UserModel> {
    return {
      username: '',
      password: '',
      status: USER_STATUS.ACTIVE,
      role: '',
      branchId: [],
      allowed_ips: [],
      userProfile: {
        fullname: '',
        phone: '',
        nickname: '',
        gender: null as any,
        departmentList: [],
        code: '',
        id_pancake: '',
      } as any,
    };
  }

  /**
   * API thay đổi mật khẩu user
   * @param changePasswordParams
   * @param options
   */
  changePassword(
    changePasswordParams: ChangePasswordModel,
    options: any = {},
  ): Observable<LoginResponse> {
    const params = this.beforeSave(changePasswordParams);
    return this.apiService.post(`${this.path}/update-password`, params, options);
  }

  /**
   * API tạo mới user
   * @param createParams
   * @param options
   */
  override create<User extends object>(createParams: User, options: any = {}): Observable<User> {
    const params = this.beforeSave(createParams);
    return this.apiService.post(`${this.path}`, params, options);
  }

  /**
   * API cập nhật user
   * @param id
   * @param updateParams
   * @param options
   */
  updateUser(
    id: number,
    updateParams: UserModel,
    options: any = {},
  ): Observable<ApiResponseOneModel<UserModel>> {
    const params = this.beforeSave(updateParams);
    return this.apiService
      .put<ApiResponseOneModel<UserModel>>(`${this.path}/${id}`, params, options)
      .pipe(
        map((res: any) => {
          const cloned = JSON.parse(JSON.stringify(res));
          cloned.data = this.afterLoad(cloned.data);
          return cloned;
        }),
      );
  }

  /**
   * API reset mật khẩu
   * @param id
   */
  resetPassword(id: number, password: string) {
    return this.apiService.put(`${this.path}/reset-password`, { id, password });
  }

  /**
   * Tải file mẫu import nhân sự.
   * File sinh bằng `go run ./cmd/gen-template -only=user` từ chính colDef export của BE — cùng bộ
   * cột, cùng thứ tự với file bấm Export ra, nên export → sửa → import lại luôn khớp.
   */
  downloadImportTemplate() {
    const a = document.createElement('a');
    a.href = '/assets/download/mau_import_nhan_su.xlsx';
    a.download = 'mau_import_nhan_su.xlsx';
    a.click();
  }

  /**
   * API import nhân sự từ file Excel/CSV
   */
  importUser(file: File): Observable<any> {
    const formData: FormData = new FormData();
    formData.append('file', file, file.name);
    return this.apiService.post(`${this.path}/import-users`, formData);
  }

  /**
   * API export danh sách nhân sự.
   * `columns` (tuỳ chọn) = danh sách `value` của displayCols đang bật — BE lọc cột theo đó và luôn
   * ép giữ cột `id` để file vẫn import ngược được.
   */
  exportUser(params: any = {}): Observable<string> {
    return this.apiService.get(`${this.path}?_export=1`, params);
  }

  addLabel(item: UserModel) {
    const itemWithDates = this.addFormattedTimestampFields(item, ['created_at', 'updated_at']);
    return {
      ...itemWithDates,
      genderLabel: getGenderLabel?.(item.userProfile?.gender!) ?? '',
      genderKey: getGenderKey(item.userProfile?.gender!),
      createdByInfo: this.userHelper.getUserInfo(item.created_by),
      updatedByInfo: this.userHelper.getUserInfo(item.updated_by),
      branchActiveLabel: this.branchHelper.getBranchName(item.userProfile?.branchActive_id),
      departmentInfo: this.departmentHelperService.mapDepartmentsNames(
        item.userProfile?.departmentList ?? [],
      ),
      branchLabel: this.branchHelper.mapBranchNames(item.branchId || []).join(', '),
      userStatusLabel: getUserStatusLabel(item.status!),
      userStatusClass: `userStatus-${item.status}`,
    };
  }

  async addLabels(items: UserModel[]) {
    if (!Array.isArray(items)) return [];
    await this.ensureCacheLoaded();
    return items.map((item) => this.addLabel(item));
  }

  async ensureCacheLoaded() {
    await Promise.all([
      this.userHelper.ensureUsersLoaded(),
      this.branchHelper.ensureBranchesLoaded(),
      this.departmentHelperService.ensureDepartmentLoaded(),
    ]);
  }
}
