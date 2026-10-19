import { ChangeDetectionStrategy, Component, DestroyRef, OnInit, inject } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { TranslatePipe, TranslateService } from '@ngx-translate/core';
import { ButtonModule } from 'primeng/button';

import { AuthService } from '@/core/services/auth/auth.service';
import { AUTH_GRADIENT_BG } from '@/core/services/auth/auth-theme';
import { UserChangePassword } from '@/modules/hr/user/form/change-password/user-change-password';
import { DialogWrapperService } from '@/share/services/dialog-wrapper.service';

/**
 * S-4/D-1 — màn đổi mật khẩu bắt buộc (cờ `user.mustChangePassword`, xem `authGuard`). Không layout,
 * không menu: user chỉ có 2 lối ra — đổi mật khẩu (tái dùng form `UserChangePassword` của menu tài
 * khoản; đổi xong BE revoke mọi phiên ⇒ form tự đăng xuất về /login) hoặc đăng xuất. BE đồng thời chặn
 * mọi API nghiệp vụ (403 `password_change_required`) nên rời trang bằng URL cũng không dùng được app.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  standalone: true,
  imports: [TranslatePipe, ButtonModule],
  template: `
    <div style="min-height:100vh;display:flex;align-items:center;justify-content:center;padding:20px;background:${AUTH_GRADIENT_BG}">
      <div class="bg-surface-0 dark:bg-surface-900" style="border-radius:20px;padding:32px;max-width:440px;width:100%;box-shadow:0 20px 60px rgba(0,0,0,.3)">
        <h2 class="text-[var(--p-text-color)]" style="font-size:1.6923rem;font-weight:700;margin-bottom:6px">
          <i class="pi pi-lock mr-2"></i>{{ 'auth.force_password.title' | translate }}
        </h2>
        <p class="text-[var(--p-text-muted-color)]" style="font-size:1.0769rem;margin-bottom:20px">{{ 'auth.force_password.desc' | translate }}</p>
        <button type="button" pButton class="w-full mb-3" (click)="openChangePassword()">
          <i class="pi pi-key"></i><span>{{ 'auth.force_password.change' | translate }}</span>
        </button>
        <button type="button" pButton severity="secondary" [outlined]="true" class="w-full" (click)="logout()">
          <i class="pi pi-sign-out"></i><span>{{ 'auth.force_password.logout' | translate }}</span>
        </button>
      </div>
    </div>
  `,
})
export class ForceChangePassword implements OnInit {
  private authService = inject(AuthService);
  private dialogWrapperService = inject(DialogWrapperService);
  private translate = inject(TranslateService);
  private destroyRef = inject(DestroyRef);

  ngOnInit(): void {
    this.openChangePassword();
  }

  openChangePassword(): void {
    this.dialogWrapperService.open(UserChangePassword, {
      width: '520px',
      header: this.translate.instant('auth.force_password.change'),
    });
  }

  logout(): void {
    this.authService.logout().pipe(takeUntilDestroyed(this.destroyRef)).subscribe();
  }
}
