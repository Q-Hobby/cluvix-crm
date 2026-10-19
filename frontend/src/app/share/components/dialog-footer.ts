import {ChangeDetectionStrategy, Component, EventEmitter, Input, Output } from '@angular/core';
import { TranslatePipe } from '@ngx-translate/core';
import { ButtonDirective } from 'primeng/button';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-dialog-footer',
  template: `
    <div
      class="sticky bottom-0 z-50 bg-[var(--p-content-background)] border-t border-[var(--p-content-border-color)] mt-4 py-3 pr-3 text-right rounded-b-xl"
    >
      <button type="button" class="mr-2" pButton severity="secondary" (click)="onCancel($event)">
        <i class="pi pi-arrow-left"></i>
        <span>{{ cancelLabel | translate }}</span>
      </button>

      @if (showSaveAndNew) {
        <button type="button" class="mr-2" pButton severity="secondary" [outlined]="true" [disabled]="isSubmitting" (click)="onSaveAndNew()">
          <i class="pi" [class.pi-spin]="isSubmitting" [class.pi-spinner]="isSubmitting" [class.pi-plus]="!isSubmitting"></i>
          <span>{{ saveAndNewLabel | translate }}</span>
        </button>
      }

      @if (showSave) {
        <button type="button" pButton severity="success" [disabled]="isSubmitting" (click)="onSave()">
          <i class="pi" [class.pi-spin]="isSubmitting" [class.pi-spinner]="isSubmitting" [class.pi-check]="!isSubmitting"></i>
          <span>{{ saveLabel | translate }}</span>
        </button>
      }
    </div>
  `,
  standalone: true,
  imports: [ButtonDirective, TranslatePipe],
})
export class DialogFooter {
  @Input() cancelLabel = 'common.action.cancel';
  @Input() saveLabel = 'common.action.save';
  @Input() isSubmitting = false;
  @Input() showSave = true;
  /**
   * Nút phụ "Lưu và tiếp tục" (lưu xong mở lại form trắng để nhập bản ghi kế tiếp).
   * Mặc định TẮT ⇒ 278 chỗ đang dùng `<app-dialog-footer>` không đổi gì. Chỉ form nào thật sự
   * có luồng nhập liên tiếp mới bật + xử lý `(saveAndNew)`.
   */
  @Input() showSaveAndNew = false;
  @Input() saveAndNewLabel = 'common.action.save_and_new';

  @Output() cancelForm = new EventEmitter<MouseEvent>();
  @Output() saveForm = new EventEmitter<void>();
  @Output() saveAndNew = new EventEmitter<void>();

  onCancel(event: MouseEvent) {
    this.cancelForm.emit(event);
  }

  onSave() {
    this.saveForm.emit();
  }

  onSaveAndNew() {
    this.saveAndNew.emit();
  }
}
