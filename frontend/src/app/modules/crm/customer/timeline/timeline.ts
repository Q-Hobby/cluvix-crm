import { TranslatePipe } from '@ngx-translate/core';
import {ChangeDetectionStrategy, Component, computed, effect, inject, input, signal } from '@angular/core';
import { TimelineModule } from 'primeng/timeline';

import { CustomerService } from '@/core/services/crm/customer.service';
import { DetailShareImports } from '@/share/import/detail-share-imports';
import { GlobalLoadingService } from '@/share/services/global-loading.service';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-customer-contact-timeline',
  templateUrl: './timeline.html',
  styleUrl: 'timeline.scss',
  standalone: true,
  imports: [...DetailShareImports, TimelineModule, TranslatePipe],
})
export class Timeline {
  customerId = input.required<number>();
  align = input<'left' | 'right' | 'alternate' | 'top' | 'bottom'>('alternate');
  private customerService = inject(CustomerService);
  private loadingService = inject(GlobalLoadingService);
  private timelineData = signal<any>(null);

  events = computed(() => {
    const data = this.timelineData();
    if (!data?.timeline) return [];

    return data.timeline.map((item: any) => ({
      ...item,
      iconDirection: item.iconDirection || 'pi pi-circle',
    }));
  });

  customer = computed(() => this.timelineData()?.customer || null);

  constructor() {
    effect(() => {
      const id = this.customerId();
      if (id) {
        this.loadingService.showLoading('timeline');
        this.customerService.getTimeLine(id).subscribe({
          next: (res) => {
            this.timelineData.set(res);
            this.loadingService.hideLoading('timeline');
          },
          error: () => {
            this.loadingService.hideLoading('timeline');
          },
        });
      }
    });
  }
}
