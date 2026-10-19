import { TranslatePipe } from '@ngx-translate/core';
import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { FormField, SchemaPathTree } from '@angular/forms/signals';
import { FloatLabel } from 'primeng/floatlabel';
import { Fluid } from 'primeng/fluid';
import { ProgressSpinnerModule } from 'primeng/progressspinner';

import { UserProfileModel } from '@/core/models/user-profile.model';
import { authUser, branchActiveId, userProfile } from '@/core/services/auth';
import { UserProfileService } from '@/core/services/config/user-profile.service';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import { DialogFooter } from '@/share/components/dialog-footer';
import { TenantService } from '@/core/services/tenant/tenant.service';
import { ReloadService } from '@/share/services/reload.service';
import { BranchCacheService, BranchFormControl } from '@/indexeddb/config/components/branch';

/**
 * story-42 (E6 bỏ company) — bản thay dialog đổi thương hiệu cũ, CHỈ còn đổi chi nhánh
 * (`branchActive_id`). BE `SwitchProfile` (story-58) đã bỏ mọi kiểm tra/ghi liên quan
 * `companyActive_id` trong payload — form này không còn gửi field đó nữa.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-switch-branch',
  templateUrl: './switch-branch.html',
  styles: [],
  imports: [
    ProgressSpinnerModule,
    FormField,
    FloatLabel,
    Fluid,
    DialogFooter,
    BranchFormControl,
    TranslatePipe],
  standalone: true,
})
export class SwitchBranch extends BaseSignalForm<UserProfileModel> {
  modelService = inject(UserProfileService);
  reloadService = inject(ReloadService);
  private tenantService = inject(TenantService);
  private branchCache = inject(BranchCacheService);
  protected readonly loadingOptions = signal(true);

  constructor() {
    super();
    this.trackScopeFields('branchActive_id');
  }

  /**
   * Load sẵn branch cache trước khi hiện form — tránh dropdown rỗng/giật khi user bấm select
   * ngay lúc cache (IndexedDB) chưa kịp load xong (BaseCacheService idempotent qua isLoaded nên
   * BranchFormControl tự load lại bên trong không tốn thêm API).
   *
   * KHÔNG gọi `super.ngOnInit()`: form không CRUD theo id. Seed model HẸP đúng 1 field.
   */
  override async ngOnInit() {
    this.loadingOptions.set(true);
    await this.branchCache.loadBranches();
    this.loadingOptions.set(false);
    const profile = userProfile();
    this.model.set({
      branchActive_id: profile?.branchActive_id ?? null,
      // `UserProfileModel` khai field là `number?` nhưng rỗng BẮT BUỘC là `null` (không được
      // `undefined` — gotcha #1). Cast qua `unknown` thay vì nới kiểu model dùng chung toàn app
      // (gotcha #24: nới kiểu model có thể vỡ AOT ở component khác).
    } as unknown as UserProfileModel);
  }

  /** Đúng 1 key — không để `id`/key thừa lọt vào payload (BE bỏ qua `companyActive_id` từ story-58). */
  protected override getPayload(): any {
    return {
      branchActive_id: this.rootField.branchActive_id!().value(),
    };
  }

  override onSubmit(): void {
    if (this.isSubmitting()) return;

    const payload = this.getPayload();
    // Chuẩn hoá `?? null` CẢ HAI VẾ: form seed rỗng bằng `null` (bắt buộc theo gotcha #1) trong khi
    // `branchActiveId()` trả `undefined` khi profile chưa set. So thẳng thì `null !== undefined`
    // → user chưa đổi gì vẫn bị gọi API + `location.reload()` toàn trang.
    const sameBranch = (payload.branchActive_id ?? null) === (branchActiveId() ?? null);

    if (!sameBranch) {
      this.isSubmitting.set(true);
      this.modelService.switchUserProfile(payload).subscribe({
        next: async (res: any) => {
          try {
            // Toast bị mất khi reload → lưu sessionStorage, hiện sau reload trong AppComponent
            const detail = (res && typeof res === 'object' && res.message) ? res.message : 'Chuyển đổi thành công';
            sessionStorage.setItem('post_reload_toast', detail);
            // Update localStorage and signal before reload — tạo object mới, không mutate trực tiếp
            const currentUser = authUser();
            if (currentUser && currentUser.userProfile) {
              const updated = {
                ...currentUser,
                userProfile: {
                  ...currentUser.userProfile,
                  branchActive_id: payload.branchActive_id,
                },
              };
              authUser.set(updated);
              this.storageService.saveStorage('auth_user', updated);
            }
            this.reloadService.sendMessage('reload');
            await this.tenantService.dispatchTenantChange();
          } catch (err) {
            console.error('[SwitchBranch] async subscribe error:', err);
          } finally {
            location.reload();
          }
        },
        error: () => {
          this.isSubmitting.set(false);
        },
      });
    } else {
      this.dialogRef?.close();
    }
  }

  /** Không validator nào — giữ đúng hành vi bản gốc (`SwitchCompany`) không khai Validators. */
  protected override buildSchema(_p: SchemaPathTree<UserProfileModel>): void {}
}
