import { ChangeDetectionStrategy, Component, Input, inject } from '@angular/core';
import { TranslatePipe } from '@ngx-translate/core';
import { TooltipModule } from 'primeng/tooltip';

import { NotificationService } from '@/share/services/notification.service';

@Component({
  selector: 'app-customer-phone',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [TooltipModule, TranslatePipe],
  template: `
    @if (phone) {
      <div
        class="cursor-pointer"
        [innerText]="phone"
        [pTooltip]="'common.tooltip.copy_phone' | translate"
        tooltipPosition="top"
        (click)="onClick()"
      ></div>
    } @else {
      —
    }
  `,
})
export class CustomerPhone {
  @Input() phone: string | null | undefined;

  private noti = inject(NotificationService);

  onClick(): void {
    if (!this.phone) return;
    navigator.clipboard.writeText(this.phone).then(() => {
      this.noti.success('Đã sao chép', this.phone!);
    });
  }
}
