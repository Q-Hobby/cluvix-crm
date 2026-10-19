import {ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { DynamicDialogConfig, DynamicDialogRef } from 'primeng/dynamicdialog';

import { CustomerModel } from '@/core/models/crm/customer.model';
import { CustomerService } from '@/core/services/crm/customer.service';
import { FormShareImports } from '@/share/import/form-share-imports';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-duplicate-code',
  templateUrl: './duplicate-code.html',
  styles: [],
  standalone: true,
  imports: [...FormShareImports],
})
export class DuplicateCode {
  duplicateModels!: CustomerModel[];
  modelService = inject(CustomerService);
  dialogRef = inject(DynamicDialogRef);
  dialogConfig = inject(DynamicDialogConfig);

  constructor() {
    this.duplicateModels = this.dialogConfig?.data?.duplicateModels;
  }
}
