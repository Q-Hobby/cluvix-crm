import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { Router } from '@angular/router';
import { finalize } from 'rxjs';
import { FormField, required, minLength, validate, SchemaPathTree } from '@angular/forms/signals';

import { ChangePasswordModel } from '@/core/models/change-password.model';
import { UserService } from '@/core/services/hr/user.service';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import { FormShareImports } from '@/share/import/form-share-imports';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './user-change-password.html',
  styles: [],
  imports: [...FormShareImports, FormField],
  standalone: true,
})
export class UserChangePassword extends BaseSignalForm<ChangePasswordModel> {
  modelService = inject(UserService);
  router = inject(Router);

  /** Seed trực tiếp — KHÔNG gọi `UserService.getInitialObject()` (shape `UserModel`, không khớp
   * `ChangePasswordModel`, xem story Dev Notes). Form này luôn "tạo mới" (không có edit-mode/id thật). */
  override ngOnInit(): void {
    this.model.set({ oldpassword: '', newpassword: '', newpasswordrepeat: '' });
  }

  override onSubmit(): void {
    if (this.isSubmitting()) return;
    this.submitted.set(true);
    this.rootField().markAsTouched();
    if (this.rootField().invalid()) return;

    this.isSubmitting.set(true);

    this.modelService
      .changePassword(this.getPayload())
      .pipe(finalize(() => this.isSubmitting.set(false)))
      .subscribe({
        next: async (_res) => {
          // Backend đã revoke toàn bộ access + refresh token — phải logout ngay
          this.noti.successMsg(this.translate.instant('hr.change_password.success_msg'));
          try { this.dialogRef?.close(true); } catch {}
          // Delay 1.5s để user đọc toast trước khi redirect về login
          setTimeout(() => this.authService.handleRefreshTokenExpired(), 1500);
        },
      });
  }

  protected override buildSchema(p: SchemaPathTree<ChangePasswordModel>): void {
    required(p.oldpassword!);
    required(p.newpassword!);
    minLength(p.newpassword!, 6);
    required(p.newpasswordrepeat!);
    // Tái đánh giá TỰ ĐỘNG khi `newpassword` đổi (declarative) — KHÔNG cần subscribe
    // `valueChanges`/`updateValueAndValidity()` thủ công như bản gốc.
    validate(p.newpasswordrepeat!, ({ value, valueOf }) =>
      value() !== valueOf(p.newpassword!) ? { kind: 'mismatch' } : null,
    );
  }
}
