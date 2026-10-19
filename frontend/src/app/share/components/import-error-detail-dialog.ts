import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { TranslatePipe } from '@ngx-translate/core';
import { ButtonDirective } from 'primeng/button';
import { DynamicDialogConfig, DynamicDialogRef } from 'primeng/dynamicdialog';

/** 1 nhóm lỗi cùng loại — khớp `response.ImportErrorGroup` phía BE. */
export interface ImportErrorGroup {
  type: string;
  label: string;
  items: string[];
}

/**
 * Dialog hiển thị TOÀN BỘ lỗi của 1 lần import (BE trả `{ message, details }` khi 422),
 * nhóm theo loại (company/material/batch_code/...) thay vì toast 1 dòng — dùng chung cho
 * mọi flow import cần báo lỗi tường minh. Data: `{ message: string, details: ImportErrorGroup[] }`.
 */
@Component({
  selector: 'app-import-error-detail-dialog',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [ButtonDirective, TranslatePipe],
  template: `
    <div class="flex flex-col gap-4 px-6 py-5 max-h-[70vh]">
      <div class="flex items-start gap-3">
        <div class="flex items-center justify-center p-2 rounded-full border shrink-0
                    bg-red-100 dark:bg-red-400/30 text-red-500 dark:text-red-200 border-red-200 dark:border-red-400/20">
          <i class="pi pi-exclamation-triangle text-2xl! leading-none!"></i>
        </div>
        <p class="text-base font-semibold text-surface-900 dark:text-surface-0 m-0">{{ message() }}</p>
      </div>

      <div class="flex-1 overflow-y-auto flex flex-col gap-4 pr-1">
        @for (group of details(); track group.type) {
          <div class="border border-surface-200 dark:border-surface-700 rounded-lg overflow-hidden">
            <div class="px-3 py-2 bg-surface-100 dark:bg-surface-800 flex items-center justify-between">
              <span class="font-semibold text-sm text-surface-800 dark:text-surface-100">{{ group.label }}</span>
              <span class="text-xs text-surface-500 dark:text-surface-400">{{ group.items.length }}</span>
            </div>
            <ul class="m-0 py-1 pl-0 list-none">
              @for (item of group.items; track $index) {
                <li class="px-3 py-1.5 text-sm text-surface-700 dark:text-surface-300 border-t border-surface-100 dark:border-surface-700 first:border-t-0">
                  {{ item }}
                </li>
              }
            </ul>
          </div>
        }
      </div>

      <div class="w-full flex items-center justify-end gap-2 pt-2">
        <button type="button" pButton severity="secondary" (click)="close()">
          <i class="pi pi-times"></i>
          <span>{{ 'common.label.close' | translate }}</span>
        </button>
      </div>
    </div>
  `,
})
export class ImportErrorDetailDialog {
  private dialogRef = inject(DynamicDialogRef);
  private config = inject(DynamicDialogConfig);

  readonly message = signal<string>(this.config.data?.message ?? '');
  readonly details = signal<ImportErrorGroup[]>(this.config.data?.details ?? []);

  close(): void {
    this.dialogRef.close();
  }
}
