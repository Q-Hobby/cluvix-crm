import {ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { DynamicDialogConfig, DynamicDialogRef } from 'primeng/dynamicdialog';

import { CustomerModel } from '@/core/models/crm/customer.model';
import { CustomerService } from '@/core/services/crm/customer.service';
import { FormShareImports } from '@/share/import/form-share-imports';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-duplicate-phone',
  templateUrl: './duplicate-phone.html',
  styles: [],
  standalone: true,
  imports: [...FormShareImports],
})
export class DuplicatePhone {
  duplicateModels!: CustomerModel[];
  /**
   * D-3: trùng SĐT là CHẶN (SĐT duy nhất trong 1 DB — BE có UNIQUE) ⇒ chỉ cho xem + huỷ, không có nút
   * "Lưu trùng". Trùng mã KH cũ vẫn chỉ cảnh báo (blocking=false).
   */
  blocking = false;
  isSubmitting: boolean = false;
  modelService = inject(CustomerService);
  dialogRef = inject(DynamicDialogRef);
  dialogConfig = inject(DynamicDialogConfig);

  constructor() {
    this.duplicateModels = this.dialogConfig?.data?.duplicateModels;
    this.blocking = !!this.dialogConfig?.data?.blocking;
  }
}
