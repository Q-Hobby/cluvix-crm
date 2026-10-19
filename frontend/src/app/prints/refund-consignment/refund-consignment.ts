import { ChangeDetectionStrategy, Component, DestroyRef, inject, OnInit, ViewEncapsulation } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { forkJoin } from 'rxjs';

import { PrintService } from '@/prints/print.service';
import { PrintHeaderLayout } from '@/layout/print/customer-info.layout';
import { BasePrint } from '@/share/base/base-print';
import { PrintShareImport } from '@/share/import/print-share-import';
import { TranslatePipe } from '@ngx-translate/core';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-refund-print-deposit-consignment-prints',
  templateUrl: './refund-consignment.html',
  styleUrls: ['../../../assets/overrides/print-order.scss'],
  encapsulation: ViewEncapsulation.None,
  standalone: true,
  imports: [...PrintShareImport, PrintHeaderLayout, TranslatePipe],
})
export class RefundConsignment extends BasePrint implements OnInit {
  private printService = inject(PrintService);
  private destroyRef = inject(DestroyRef);

  ngOnInit(): void {
    this.activatedRoute.params.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((queryParams: any) => {
      const cacheKey = this.activatedRoute.snapshot.queryParams['cacheKey'];
      const cached = this.tryLoadFromCache(cacheKey);
      if (cached) { this.applyData(cached); return; }
      forkJoin({
        model: this.printService.refundConsignment(queryParams.id),
      }).subscribe(({ model }: any) => this.applyData(model));
    });
  }

  applyData(model: any) {
    const data = model?.data ?? model;
    this.model.set(data);
    // fe-print.md §3 (CRITICAL): branch falsy → không in được, không loading, không báo lỗi.
    this.printData.setModel({
      branch: data?.branch,
      customer: data?.customer,
    });
    setTimeout(() => void this.handlePrintAfterInit());
  }

  protected override async handlePrintAfterInit(): Promise<void> {
    const d = this.model()?.payment_time;
    await this.formatDate(d ? new Date(d * 1000) : new Date());
  }

  formatNewDate($date: any) { return new Date($date); }
}
