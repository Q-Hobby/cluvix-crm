import { CommonModule } from '@angular/common';
import { ChangeDetectionStrategy, Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { TranslatePipe } from '@ngx-translate/core';

import { CustomerModel } from '@/core/models/crm/customer.model';
import { PhoneFormatPipe } from '@/share/pipes/phone-format/phone-format.pipe';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-customer-info-layout',
  standalone: true,
  template: `
    <div class="grid grid-cols-1 md:grid-cols-2 mt-2 text-gray-700 print:grid-cols-2 mb-2">
      @if (customer?.fullname) {
        <div>
          {{ 'common.label.customer' | translate }}:
          <b class="text-gray-900">{{ customer!.fullname! }}</b>
        </div>
      }

      @if (customer?.phone) {
        <div>
          {{ 'common.label.phone' | translate }}:
          <b class="text-gray-900">{{ customer!.phone! | phoneFormat }}</b>
        </div>
      }

      @if (customer?.address) {
        <div class="col-span-2">
          {{ 'common.label.address' | translate }}:
          <b class="text-gray-900">{{ customer!.address }}</b>
        </div>
      }
    </div>
  `,
  imports: [CommonModule, TranslatePipe, PhoneFormatPipe],
})
export class PrintHeaderLayout implements OnInit {
  @Input() customer?: CustomerModel | null;
  @Output() logoLoaded = new EventEmitter<void>();

  ngOnInit() {
    // Emit logoLoaded since this component doesn't have actual logo
    setTimeout(() => {
      this.logoLoaded.emit();
    }, 100);
  }
}
