import {ChangeDetectionStrategy, Component, DestroyRef, inject, OnInit, ViewEncapsulation} from '@angular/core';
import {takeUntilDestroyed} from '@angular/core/rxjs-interop';
import {switchMap} from 'rxjs';

import {PaymentService} from '@/core/services/crm/payment.service';
import {PrintService} from '@/prints/print.service';
import {PrintHeaderLayout} from '@/layout/print/customer-info.layout';
import {BasePrint} from '@/share/base/base-print';
import {PrintShareImport} from '@/share/import/print-share-import';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './print-deposit.html',
  styleUrls: ['./print-deposit.scss'],
  encapsulation: ViewEncapsulation.None,
  imports: [...PrintShareImport, PrintHeaderLayout],
  standalone: true,
})
export class PrintDeposit extends BasePrint implements OnInit {
  modelService = inject(PaymentService);
  private printService = inject(PrintService);
  private destroyRef = inject(DestroyRef);

  ngOnInit() {
    this.activatedRoute.params.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((queryParams: any) => {
      const cacheKey = this.activatedRoute.snapshot.queryParams['cacheKey'];
      const cached = this.tryLoadFromCache(cacheKey);
      if (cached) {
        this.applyRaw(cached);
        return;
      }
      this.printService.deposit(queryParams.id)
        .pipe(switchMap(async (raw: any) => {
          await this.modelService.ensureCacheLoaded();
          return this.modelService.addLabel(raw);
        }))
        .subscribe((model: any) => this.applyData(model));
    });
  }

  // Dùng khi có cache (raw model từ sessionStorage, chưa addLabel)
  async applyRaw(rawModel: any) {
    await this.modelService.ensureCacheLoaded();
    this.applyData(this.modelService.addLabel(rawModel));
  }

  applyData(model: any) {
    this.model.set(model);
    this.printData.setModel({
      branch: model.branch_id,
      customer: model.customer,
      layoutType: 'with-accountant',
    });
    setTimeout(() => void this.handlePrintAfterInit());
  }

  protected override async handlePrintAfterInit(): Promise<void> {
    const d = this.model()?.payment_time;
    await this.formatDate(d ? new Date(d * 1000) : new Date());
  }
}
