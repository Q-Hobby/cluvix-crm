import { ChangeDetectionStrategy, Component, DestroyRef, inject, OnInit, ViewEncapsulation } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';

import { PrintService } from '@/prints/print.service';
import { PrintHeaderLayout } from '@/layout/print/customer-info.layout';
import { BasePrint } from '@/share/base/base-print';
import { PrintShareImport } from '@/share/import/print-share-import';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-print-payment-prints-print-deposit',
  templateUrl: './refund-deposit.html',
  styleUrls: ['../../../assets/overrides/print-order.scss'],
  encapsulation: ViewEncapsulation.None,
  standalone: true,
  imports: [...PrintShareImport, PrintHeaderLayout],
})
export class RefundDeposit extends BasePrint implements OnInit {
  private printService = inject(PrintService);
  private destroyRef = inject(DestroyRef);

  ngOnInit() {
    this.activatedRoute.params.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((queryParams: any) => {
      const cacheKey = this.activatedRoute.snapshot.queryParams['cacheKey'];
      const cached = this.tryLoadFromCache(cacheKey);
      if (cached) { this.applyData(cached); return; }
      this.printService.refundDeposit(queryParams.id)
        .subscribe((model: any) => this.applyData(model));
    });
  }

  applyData(model: any) {
    this.model.set(model?.data ?? model);
    this.printData.setModel({
      branch: this.model().branch_id,
      customer: this.model().customer,
    });
    setTimeout(() => void this.handlePrintAfterInit());
  }

  protected override async handlePrintAfterInit(): Promise<void> {
    const d = this.model()?.payment_time;
    await this.formatDate(d ? new Date(d * 1000) : new Date());
  }
}
