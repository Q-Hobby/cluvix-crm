import { ChangeDetectionStrategy, Component, DestroyRef, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { finalize } from 'rxjs';
import { DynamicDialogConfig } from 'primeng/dynamicdialog';

import { ServiceService } from '@/core/services/config/clinic/service.service';
import { BaseApp } from '@/share/base/BaseApp';
import { FormShareImports } from '@/share/import/form-share-imports';

@Component({
  selector: 'app-service-import-dialog',
  templateUrl: './service-import-dialog.html',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [...FormShareImports],
})
export class ServiceImportDialog extends BaseApp {
  private modelService = inject(ServiceService);
  private destroyRef = inject(DestroyRef);
  private dialogConfig = inject(DynamicDialogConfig, { optional: true });

  protected readonly selectedFile = signal<File | null>(null);
  protected readonly importResult = signal<{ created: number; updated: number; skipped: number } | null>(null);

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
    this.loadingService.showLoading('serviceImport');
    this.modelService
      .importServiceFromExcel(file)
      .pipe(
        takeUntilDestroyed(this.destroyRef),
        finalize(() => {
          this.isSubmitting.set(false);
          this.loadingService.hideLoading('serviceImport');
        }),
      )
      .subscribe({
        next: (res: any) => {
          this.importResult.set(res?.data ?? { created: 0, updated: 0, skipped: 0 });
          this.dialogConfig?.data?.onImported?.();
        },
      });
  }

  downloadTemplate() {
    this.modelService.downloadImportTemplate();
  }

  cancel() {
    this.dialogRef.close(!!this.importResult());
  }
}
