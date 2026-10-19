import { TranslatePipe } from '@ngx-translate/core';
import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { HttpContext } from '@angular/common/http';
import { FormField, SchemaPathTree } from '@angular/forms/signals';
import { FloatLabel } from 'primeng/floatlabel';
import { Select } from 'primeng/select';
import { finalize } from 'rxjs';

import { UserProfileModel } from '@/core/models/user-profile.model';
import { authUser, userProfile } from '@/core/services/auth';
import { UserProfileService } from '@/core/services/config/user-profile.service';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import { CONFIG_LOCALE_LANGUAGE_OPTIONS, STORAGE_KEY_SELECT } from '@/define';
import { LOADING_LABEL_OVERRIDE } from '@/core/interceptors/loading-label.util';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-switch-language',
  templateUrl: './switch-language.html',
  styles: [],
  imports: [FormField, Select, FloatLabel, TranslatePipe],
  standalone: true,
})
export class SwitchLanguage extends BaseSignalForm<UserProfileModel> {
  override modelService = inject(UserProfileService);
  protected readonly CONFIG_LOCALE_LANGUAGE_OPTIONS = CONFIG_LOCALE_LANGUAGE_OPTIONS;

  /**
   * Seed model HẸP đúng 1 field `locale` — KHÔNG `model.set(userProfile())` vì object profile đầy field
   * `undefined`, Signal Forms không bind được field giá trị `undefined` (gotcha #1 `signal-forms.md`).
   * Cũng KHÔNG gọi `super.ngOnInit()`: form này không CRUD theo id, không cần `getInitialObject()`.
   */
  override ngOnInit(): void {
    this.model.set({ locale: userProfile()?.locale ?? '' } as UserProfileModel);
  }

  /** Class CSS sprite cờ quốc gia (`flags.css`) — cùng convention với login.ts + sale/opportunity/portrait. */
  flagClass(locale: string): string {
    const country = locale?.split('-')[1]?.toLowerCase() ?? '';
    return `flag flag-${country}`;
  }

  selectedLanguage(event: any) {
    // CỐ Ý set tường minh dù `[formField]` cũng đồng bộ: `onSubmit()` gọi ngay dòng dưới đọc value từ
    // field, không phụ thuộc thứ tự giữa CVA writeback và `(onChange)` của `p-select`.
    this.rootField.locale!().value.set(event);
    if (userProfile()?.locale !== event) {
      this.onSubmit();
    }
  }

  /** Payload gửi dạng FormData — whitelist đúng 1 key `locale`, giữ nguyên tập key bản Reactive Forms. */
  protected override getPayload(): any {
    return { locale: this.rootField.locale!().value() };
  }

  override onSubmit(): void {
    if (this.isSubmitting()) return;
    this.submitted.set(true);
    this.rootField().markAsTouched();

    if (this.rootField().invalid()) return;

    this.isSubmitting.set(true);
    // URL `update-user-profile` dùng chung với sửa hồ sơ thường (user-update-profile.ts) — override
    // labelKey qua context để loader ghi "Đang chuyển..." thay vì "Đang tạo mới" (generic POST).
    this.modelService
      .updateUserProfile(
        this.getPayload(),
        { context: new HttpContext().set(LOADING_LABEL_OVERRIDE, 'common.label.switching') },
        true,
      )
      .pipe(
        finalize(() => {
          this.isSubmitting.set(false);
        }),
      )
      .subscribe({
        next: (updatedProfile: any) => {
          const newLocale = updatedProfile.locale || this.rootField.locale!().value();
          // Sync localStorage 'lng' để initializeTranslations() sau reload load đúng ngôn ngữ mới —
          // nếu không sync, TranslateService load locale cũ → post_reload_toast hiện sai ngữ.
          this.storageService.saveStorage(STORAGE_KEY_SELECT.Lang, newLocale);
          sessionStorage.setItem('post_reload_toast', 'common.message.switch_language_success');
          // Cập nhật authUser signal trước khi reload
          const currentUser = authUser();
          if (currentUser) {
            authUser.set({
              ...currentUser,
              userProfile: {
                ...currentUser.userProfile,
                locale: newLocale,
              },
            });
          }
          setTimeout(() => location.reload(), 100);
        },
      });
  }

  /** Không validator nào — giữ đúng hành vi bản Reactive Forms (`buildForm` cũ không khai Validators). */
  protected override buildSchema(_p: SchemaPathTree<UserProfileModel>): void {}
}
