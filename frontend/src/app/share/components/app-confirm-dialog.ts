import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { TranslatePipe } from '@ngx-translate/core';
import { ButtonDirective, ButtonSeverity } from 'primeng/button';
import { DynamicDialogConfig, DynamicDialogRef } from 'primeng/dynamicdialog';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-confirm-dialog',
  standalone: true,
  imports: [ButtonDirective, TranslatePipe],
  template: `
    <div class="py-6 px-8 flex flex-col gap-6">
      <div class="flex flex-col md:flex-row items-start gap-4">
        <div
          class="flex items-center justify-center p-2 rounded-full border shrink-0"
          [class]="iconBg()"
        >
          <i [class]="icon() + ' text-4xl! leading-none!'"></i>
        </div>
        <div class="flex flex-col gap-3">
          @if (title()) {
            <span class="font-semibold text-xl text-surface-900 dark:text-surface-0">{{ title() }}</span>
          }
          <p class="text-base leading-normal text-surface-700 dark:text-surface-300 p-0 m-0">{{ message() }}</p>
        </div>
      </div>
      <div class="w-full flex items-center justify-end gap-2">
        <button type="button" pButton [outlined]="true" (click)="reject()">
          <span>{{ rejectLabel() | translate }}</span>
        </button>
        <button type="button" pButton [severity]="acceptSeverity()" (click)="accept()">
          <i class="pi pi-check"></i>
          <span>{{ acceptLabel() | translate }}</span>
        </button>
      </div>
    </div>
  `,
})
export class AppConfirmDialog {
  private dialogRef = inject(DynamicDialogRef);
  private config = inject(DynamicDialogConfig);

  readonly message  = signal<string>(this.config.data?.message ?? '');
  readonly title    = signal<string>(this.config.data?.title ?? '');
  readonly icon     = signal<string>(this.config.data?.icon ?? 'pi pi-exclamation-triangle');
  readonly iconBg   = signal<string>(
    this.config.data?.iconBg ??
    'bg-orange-100 dark:bg-orange-400/30 text-orange-500 dark:text-orange-200 border-orange-200 dark:border-orange-400/20',
  );
  /**
   * Màu nút xác nhận. Mặc định `success` — GIỮ NGUYÊN hành vi của 20 chỗ đang gọi.
   *
   * Truyền `'danger'` cho thao tác PHÁ HUỶ (xoá, buộc ngắt): nút xanh lá đọc là "an toàn, cứ đi
   * tiếp", ngược hẳn với việc sắp làm — và icon cảnh báo đỏ bên trái càng làm hai tín hiệu đá nhau.
   */
  readonly acceptSeverity = signal<ButtonSeverity>(this.config.data?.acceptSeverity ?? 'success');
  readonly acceptLabel = signal<string>(this.config.data?.acceptLabel ?? 'common.action.confirm');
  readonly rejectLabel = signal<string>(this.config.data?.rejectLabel ?? 'common.action.cancel');

  accept(): void { this.dialogRef.close(true); }
  reject(): void { this.dialogRef.close(false); }
}
