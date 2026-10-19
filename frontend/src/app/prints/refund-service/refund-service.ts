import { ChangeDetectionStrategy, Component, DestroyRef, inject, OnInit, ViewEncapsulation } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';

import { PrintService } from '@/prints/print.service';
import { PrintHeaderLayout } from '@/layout/print/customer-info.layout';
import { BasePrint } from '@/share/base/base-print';
import { PrintShareImport } from '@/share/import/print-share-import';
// [appCopyText] dùng ở template (dòng 57) — directive này nằm trong bộ import màn danh sách,
// KHÔNG có trong PrintShareImport nên phải import thẳng.
import { CopyTextDirective } from '@/share/directive/copy-text.directive';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-print-payment-prints-print-deposit',
  templateUrl: './refund-service.html',
  styleUrls: ['../../../assets/overrides/print-order.scss'],
  encapsulation: ViewEncapsulation.None,
  standalone: true,
  // fe-print.md §8: trang in dùng bộ tối thiểu, KHÔNG kéo cả bộ import của màn danh sách (quá nặng).
  imports: [...PrintShareImport, PrintHeaderLayout, CopyTextDirective],
})
export class RefundService extends BasePrint implements OnInit {
  private printService = inject(PrintService);
  private destroyRef = inject(DestroyRef);

  ngOnInit() {
    this.activatedRoute.params.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((queryParams: any) => {
      const cacheKey = this.activatedRoute.snapshot.queryParams['cacheKey'];
      const cached = this.tryLoadFromCache(cacheKey);
      if (cached) { this.applyData(cached); return; }
      this.printService.refundService(queryParams.id)
        .subscribe((model: any) => this.applyData(model));
    });
  }

  applyData(model: any) {
    const data = model?.data ?? model;
    this.model.set(data);
    this.printData.setModel({
      branch: data.branch_id,
      customer: data.customer,
    });
    this.models.set([data]);
    setTimeout(() => void this.handlePrintAfterInit());
  }

  protected override async handlePrintAfterInit(): Promise<void> {
    const d = this.model()?.payment_time;
    await this.formatDate(d ? new Date(d * 1000) : new Date());
  }
}
