import {ChangeDetectionStrategy, Component, inject, Input } from '@angular/core';
import { DialogModule } from 'primeng/dialog';
import { DynamicDialogConfig } from 'primeng/dynamicdialog';

import { Timeline } from '@/modules/crm/customer/timeline/timeline';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-contact-list-timeline-dialog',
  templateUrl: './customer-timeline.html',
  standalone: true,
  imports: [Timeline, DialogModule],
})
export class CustomerTimeline {
  @Input() customerId: number | undefined;
  // @Optional() giúp Angular không báo lỗi nếu DynamicDialogConfig không tồn tại. Trong trường hợp dùng component
  protected dialogConfig = inject(DynamicDialogConfig, { optional: true });

  constructor() {
    // Nếu dùng trong dialog, lấy từ dialogConfig
    // Nếu dùng như component thông thường, customerId sẽ được truyền qua @Input
    if (this.dialogConfig?.data?.customerId) {
      this.customerId = this.dialogConfig?.data?.customerId;
    }
  }
}
