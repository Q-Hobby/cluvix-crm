import { ChangeDetectionStrategy, Component, Input, OnInit } from '@angular/core';
import { TranslatePipe } from '@ngx-translate/core';
import { RouterModule } from '@angular/router';
import { TooltipModule } from 'primeng/tooltip';

import { CopyTextDirective } from '@/share/directive/copy-text.directive';
import { writeCustomerPreload } from '@/share/utils/customer-preload.util';

@Component({
  selector: 'app-customer-name',
  standalone: true,
  imports: [RouterModule, TooltipModule, CopyTextDirective, TranslatePipe],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="inline-flex items-center gap-2">
      <!--        [target]="isMobile ? '_self' : '_blank'"-->
      <a
        [routerLink]="routerPath"
        (mousedown)="onLinkInteract()"
        (auxclick)="onLinkInteract()"
        class="text-blue-600 hover:underline font-medium"
      >
        @if (name) {
          {{ name }}
        } @else {
          <i class="pi pi-external-link text-muted-color"></i>
        }
      </a>
      @if (name) {
        <i
          class="pi pi-clone text-muted-color hover:text-primary transition-all"
          [appCopyText]="name"
          [pTooltip]="'common.action.copy_name' | translate"
          tooltipPosition="top"
        >
        </i>
      }
    </div>
  `,
})
export class CustomerName implements OnInit {
  @Input({ required: true }) id!: number | string;
  @Input({ required: true }) name: string | null | undefined;
  /** Optional — nếu pass, click sẽ preload customer cho detail page paint instant. */
  @Input() customer?: any;

  isMobile = /iPhone|Android/.test(navigator.userAgent) && !/iPad/.test(navigator.userAgent);
  routerPath: string[] = [];

  ngOnInit(): void {
    this.routerPath = ['/customer', 'detail', String(this.id)];
  }

  onLinkInteract(): void {
    // Customer detail mở tab mới (target="_blank") → không truyền được history.state.
    // Cross-tab bridge: ghi sessionStorage trước khi browser mở tab → detail page đọc lúc init.
    // Chỉ ghi khi caller opt-in qua [customer].
    if (!this.customer || !this.id) return;
    writeCustomerPreload(this.id, this.customer);
  }
}
