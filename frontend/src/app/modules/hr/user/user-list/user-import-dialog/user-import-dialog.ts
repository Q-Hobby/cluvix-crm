import { ChangeDetectionStrategy, Component, DestroyRef, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { finalize } from 'rxjs';
import { DynamicDialogConfig } from 'primeng/dynamicdialog';

import { UserService } from '@/core/services/hr/user.service';
import { BaseApp } from '@/share/base/BaseApp';
import { FormShareImports } from '@/share/import/form-share-imports';

export interface ImportedUser {
  username: string;
  password: string;
}

interface UserImportResult {
  /** Tài khoản mới kèm mật khẩu sinh tự động — chỉ hiện ĐÚNG một lần này, BE không đọc lại được */
  created: ImportedUser[];
  createdCount: number;
  updated: number;
  skipped: number;
}

@Component({
  selector: 'app-user-import-dialog',
  templateUrl: './user-import-dialog.html',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [...FormShareImports],
})
export class UserImportDialog extends BaseApp {
  private modelService = inject(UserService);
  private destroyRef = inject(DestroyRef);
  private dialogConfig = inject(DynamicDialogConfig, { optional: true });

  protected readonly selectedFile = signal<File | null>(null);
  protected readonly importResult = signal<UserImportResult | null>(null);

  onFileChange(event: Event) {
    const input = event.target as HTMLInputElement;
    this.selectedFile.set(input.files?.[0] ?? null);
  }

  submit() {
    const file = this.selectedFile();
    if (!file) {
      this.noti.warnMsg(this.translate.instant('sale.lead.import_no_file'));
      return;
    }
    this.isSubmitting.set(true);
    this.loadingService.showLoading('userImport');
    this.modelService
      .importUser(file)
      .pipe(
        takeUntilDestroyed(this.destroyRef),
        finalize(() => {
          this.isSubmitting.set(false);
          this.loadingService.hideLoading('userImport');
        }),
      )
      .subscribe({
        next: (res: any) => {
          const data = res?.data ?? {};
          const created: ImportedUser[] = data.created ?? [];
          this.importResult.set({
            created,
            createdCount: data.createdCount ?? created.length,
            updated: data.updated ?? 0,
            skipped: data.skipped ?? 0,
          });
          this.dialogConfig?.data?.onImported?.();
        },
      });
  }

  downloadTemplate() {
    this.modelService.downloadImportTemplate();
  }

  /**
   * Clipboard API chỉ tồn tại trên secure context và có thể bị policy chặn — không bắt lỗi thì user
   * bấm Copy mà không nhận được phản hồi nào, tưởng đã copy được rồi đóng dialog là mất mật khẩu.
   */
  copyToClipboard(text: string) {
    navigator.clipboard
      ?.writeText(text)
      .then(() => this.noti.successMsg(this.translate.instant('common.message.copy_success')))
      .catch(() => this.noti.warnMsg(this.translate.instant('common.message.copy_failed')));
  }

  /** Copy cả danh sách tài khoản mới — bảng dài thì bấm từng dòng rất cực. */
  copyAll() {
    const rows = this.importResult()?.created ?? [];
    if (!rows.length) return;
    this.copyToClipboard(rows.map((u) => `${u.username} / ${u.password}`).join('\n'));
  }

  cancel() {
    this.dialogRef.close(!!this.importResult());
  }
}
