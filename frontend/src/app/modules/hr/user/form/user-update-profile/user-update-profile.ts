import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { finalize } from 'rxjs';
import { FormField, required, minLength, maxLength, SchemaPathTree } from '@angular/forms/signals';
import { extractValue } from '@angular/forms/signals/compat';

import { UserProfileModel } from '@/core/models/user-profile.model';
import { authUser, userId, userProfile, userRoleName } from '@/core/services/auth';
import { UserProfileService } from '@/core/services/config/user-profile.service';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import { CONFIG_FILE_SELECT, GENDER_OPTIONS, ROLE_SELECT } from '@/define';
import { FormShareImports } from '@/share/import/form-share-imports';
import { isAllowExtensions } from '@/share/services/utils.service';
import { translateOptions } from '@/share/utils/translate-options.util';
import { DepartmentHelperService } from '@/indexeddb/config/components/hr-department';

/**
 * Form user TỰ sửa hồ sơ cá nhân (khác `UserForm` story-08 — HR sửa NGƯỜI KHÁC). Đặc thù:
 * - `ngOnInit()` override HOÀN TOÀN — đọc thẳng `userProfile()` (signal global auth state, KHÔNG qua
 *   `dialogConfig.data.model`/GET), KHÔNG theo khuôn `id`/`data.model` chuẩn của `BaseSignalForm`.
 * - `avatarFile: File | null` giữ NGOÀI `FieldTree` (mirror `ServiceForm.fileImage`) — bản gốc có control
 *   `avatarFile` TRONG `FormGroup` nhưng chỉ để mang giá trị `File`, không có ý nghĩa validate/hiển thị
 *   qua `[formField]`; đưa ra ngoài là hiện đại hoá hợp lý, giữ nguyên hành vi.
 * - Story-61: `app-branch-form-control` đã bỏ input `companyId` — form này không còn lọc chi nhánh theo
 *   thương hiệu (hiện toàn bộ branch, như hành vi cũ khi `companyId` luôn `undefined`).
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './user-update-profile.html',
  styles: [
    `
      .avatar-image {
        width: 15%;
        height: 30%;
      }
    `,
  ],
  standalone: true,
  imports: [...FormShareImports, FormField],
})
export class UserUpdateProfile extends BaseSignalForm<UserProfileModel> {
  modelService = inject(UserProfileService);
  departmentHelper = inject(DepartmentHelperService);
  maxDate: Date = this.timezoneService.todayEnd();
  avatarFile: File | null = null;
  srcImageUpload: string | null = null;
  uploadError = signal<string | null>(null);
  imageLoading = signal(false);
  protected readonly RoleDef = ROLE_SELECT;
  protected readonly userRoleName = userRoleName;
  protected readonly CONFIG_FILE_SELECT = CONFIG_FILE_SELECT;
  protected readonly genderOptions = translateOptions(GENDER_OPTIONS);

  constructor() {
    super();
    const idParam = this.dialogConfig?.data?.userId ?? userId();
    this.id.set(idParam);
  }

  override ngOnInit(): void {
    const profile = this.modelService.processForForm(userProfile()!);
    this.model.set(profile);
    if (profile.avatar) {
      this.imageLoading.set(true);
      this.srcImageUpload = profile.avatar;
    }
  }

  getDepartmentLabel(departmentIds: number[] | null): string {
    if (!Array.isArray(departmentIds)) return '-';

    return this.departmentHelper.mapDepartmentsNames(departmentIds);
  }

  onImageSelect(event: any) {
    const file = event.files?.[0];
    if (!file) return;

    this.uploadError.set(null);

    if (file.size > CONFIG_FILE_SELECT.CONFIG_FILE_MAX_SIZE) {
      this.uploadError.set(this.translate.instant('hr.user_profile_form.err_max_size'));
      event.currentFiles = [];
      return;
    }

    if (!isAllowExtensions(file.name, CONFIG_FILE_SELECT.CONFIG_FILE_ALLOW_EXT, file.type)) {
      this.uploadError.set(
        this.translate.instant('hr.user_profile_form.err_invalid_ext', { exts: CONFIG_FILE_SELECT.CONFIG_FILE_ALLOW_EXT.join(', ') }),
      );
      event.currentFiles = [];
      return;
    }

    this.avatarFile = file;
  }

  onImageRemove() {
    this.avatarFile = null;
    this.uploadError.set(null);
  }

  onImageClear() {
    this.avatarFile = null;
    this.uploadError.set(null);
  }

  clearExistingImage() {
    this.srcImageUpload = null;
    this.avatarFile = null;
    this.uploadError.set(null);
    this.rootField.avatar!().value.set(null as any);
  }

  formatSize(bytes: number): string {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return Math.round((bytes / Math.pow(k, i)) * 100) / 100 + ' ' + sizes[i];
  }

  onImageLoad() {
    setTimeout(() => this.imageLoading.set(false), 500);
  }

  override onSubmit(): void {
    if (this.isSubmitting()) return;
    this.submitted.set(true);
    this.rootField().markAsTouched();
    if (this.rootField().invalid()) return;

    this.isSubmitting.set(true);
    this.modelService
      .updateUserProfile(this.getPayload(), {}, true)
      .pipe(
        finalize(() => {
          this.isSubmitting.set(false);
        }),
      )
      .subscribe({
        next: (res) => {
          this.noti.successMsg(this.translate.instant('hr.user_profile_form.update_success'));
          const currentUser = authUser();
          if (currentUser && res) {
            // Parse FormData response (BE trả về string)
            const normalized = {
              ...res,
              gender: res.gender != null ? +res.gender : null,
              branchOffice_id: res.branchOffice_id != null ? +res.branchOffice_id : null,
              birthday: res.birthday ? new Date(res.birthday + 'T00:00:00') : null,
            };
            const updatedProfile = { ...currentUser.userProfile, ...normalized };
            authUser.set({ ...currentUser, userProfile: updatedProfile as any });
          }
          this.dialogRef.close(res);
        },
      });
  }

  protected override buildSchema(p: SchemaPathTree<UserProfileModel>): void {
    required(p.fullname!);
    required(p.phone!);
    minLength(p.phone!, 10);
    maxLength(p.phone!, 12);
    required(p.gender!);
  }

  /** `avatarFile` sống ngoài `FieldTree` (mirror `ServiceForm.fileImage`) — merge vào payload lúc
   * submit. Bản gốc LUÔN gửi FormData (`isToFormData=true` hardcode), không điều kiện theo có file
   * hay không — giữ nguyên, KHÔNG áp dụng pattern `hasFile` của `ServiceForm` (form khác, quyết định
   * khác). */
  protected override getPayload(): any {
    return { ...extractValue(this.rootField), avatarFile: this.avatarFile };
  }
}
