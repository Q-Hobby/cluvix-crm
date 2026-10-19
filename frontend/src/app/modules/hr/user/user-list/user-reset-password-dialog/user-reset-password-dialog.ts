import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { TranslatePipe, TranslateService } from '@ngx-translate/core';
import { DynamicDialogConfig, DynamicDialogRef } from 'primeng/dynamicdialog';
import { InputTextModule } from 'primeng/inputtext';
import { TooltipModule } from 'primeng/tooltip';

import { UserService } from '@/core/services/hr/user.service';
import { NotificationService } from '@/share/services/notification.service';
import { UtilsService } from '@/share/services/utils.service';
import { CopyTextDirective } from '@/share/directive/copy-text.directive';
import { DialogFooter } from '@/share/components/dialog-footer';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-user-reset-password-dialog',
  templateUrl: './user-reset-password-dialog.html',
  standalone: true,
  imports: [
    ReactiveFormsModule,
    DialogFooter,
    InputTextModule,
    TooltipModule,
    TranslatePipe,
    CopyTextDirective,
  ],
})
export class UserResetPasswordDialog {
  private dialogConfig = inject(DynamicDialogConfig);
  protected dialogRef = inject(DynamicDialogRef);
  private fb = inject(FormBuilder);
  private userService = inject(UserService);
  private noti = inject(NotificationService);
  private translate = inject(TranslateService);
  private utilsService = inject(UtilsService);

  protected isSubmitting = signal(false);

  protected form = this.fb.group({
    password: [this.utilsService.randomString(), [Validators.required, Validators.minLength(6)]],
  });

  get passwordControl() {
    return this.form.controls.password;
  }

  generatePassword() {
    this.form.patchValue({ password: this.utilsService.randomString() });
  }

  onSubmit() {
    if (this.form.invalid || this.isSubmitting()) return;
    const userId: number = this.dialogConfig.data?.userId;
    if (!userId) return;
    const password = this.passwordControl.value!;
    this.isSubmitting.set(true);
    this.userService.resetPassword(userId, password).subscribe({
      next: () => {
        this.noti.successMsg(this.translate.instant('hr.user_list_page.reset_password_success'));
        this.dialogRef.close(true);
      },
      error: () => {
        this.isSubmitting.set(false);
      },
    });
  }
}
