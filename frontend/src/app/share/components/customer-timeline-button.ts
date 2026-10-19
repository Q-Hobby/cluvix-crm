import {ChangeDetectionStrategy, Component, EventEmitter, Input, Output } from '@angular/core';
import { TranslatePipe } from '@ngx-translate/core';
import { ButtonDirective } from 'primeng/button';
import { RippleModule } from 'primeng/ripple';
import { TooltipModule } from 'primeng/tooltip';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-customer-timeline-button',
  standalone: true,
  imports: [ButtonDirective, RippleModule, TooltipModule, TranslatePipe],
  template: `
    <button
      pButton
      type="button"
      (click)="handleClick($event)"
      size="small"
      severity="info"
      [iconOnly]="true"
      [pTooltip]="'crm.customer.timeline_tooltip' | translate"
      tooltipPosition="left"
    ><i class="pi pi-sitemap"></i></button>
  `,
})
export class CustomerTimelineButton {
  @Input({ required: true }) customer!: { id: number; fullname: string };
  @Output() clickTimeLine = new EventEmitter<any>();

  handleClick(event: Event) {
    this.clickTimeLine.emit({ event, ...this.customer });
  }
}
