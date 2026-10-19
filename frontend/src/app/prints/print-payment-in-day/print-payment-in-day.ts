import {ChangeDetectionStrategy, Component, DestroyRef, inject, OnInit, ViewEncapsulation} from '@angular/core';
import {takeUntilDestroyed} from '@angular/core/rxjs-interop';

import {PrintService} from '@/prints/print.service';
import {BasePrint} from '@/share/base/base-print';
import {PrintShareImport} from '@/share/import/print-share-import';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-prints-print-payment-statistics',
  templateUrl: './print-payment-in-day.html',
  styleUrls: ['../../../assets/overrides/print-order.scss'],
  standalone: true,
  encapsulation: ViewEncapsulation.None,
  imports: [...PrintShareImport],
})
export class PrintPaymentInDay extends BasePrint implements OnInit {
  private printService = inject(PrintService);
  private destroyRef = inject(DestroyRef);

  ngOnInit(): void {
    this.activatedRoute.params.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((params: any) => {
      const dateParam = params.datePrint;
      const cacheKey = this.activatedRoute.snapshot.queryParams['cacheKey'];
      const cached = this.tryLoadFromCache(cacheKey);
      if (cached) {
        this.applyData(cached);
        return;
      }
      this.printService.paymentInDay({
        params: {
          '_ngay_thang[gte]': dateParam,
          '_ngay_thang[lte]': parseInt(dateParam) + 86399,
        },
      }).subscribe((response: any) => this.applyData(response));
    });
  }

  applyData(response: any): void {
    const items: any[] = response ?? [];
    const first = items[0] ?? {};
    this.models.set(items);
    this.model.set({
      ngayThang: first.dayCreate ? first.dayCreate * 1000 : null,
      dongY: first.numAppOk ?? 0,
      doanhSo: first.totalDoanhSo ?? 0,
      thucThu: first.totalThucThu ?? 0,
      datCoc: first.totalDeposit ?? 0,
    });
    this.printData.setModel({
      layoutType: 'with-accountant',
    });
    setTimeout(() => void this.handlePrintAfterInit());
  }
}
