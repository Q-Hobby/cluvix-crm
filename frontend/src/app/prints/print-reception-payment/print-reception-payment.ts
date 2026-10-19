import { ChangeDetectionStrategy, Component, DestroyRef, inject, OnInit, ViewEncapsulation } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';

import { BasePrint } from '@/share/base/base-print';
import { PrintShareImport } from '@/share/import/print-share-import';
import { PaymentService } from '@/core/services/crm/payment.service';
import { PrintService } from '@/prints/print.service';
import { filter, firstValueFrom, switchMap } from 'rxjs';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-print-print-reception-payment',
  templateUrl: './print-reception-payment.html',
  styleUrls: ['./print-reception-payment.scss'],
  encapsulation: ViewEncapsulation.None,
  imports: [...PrintShareImport],
  standalone: true,
})
export class PrintReceptionPayment extends BasePrint implements OnInit {
  modelService = inject(PaymentService);
  private printService = inject(PrintService);
  totalAmount = 0;
  private destroyRef = inject(DestroyRef);

  ngOnInit() {
    this.activatedRoute.queryParamMap
      .pipe(
        filter((paramMap: any) => {
          if (paramMap.has('cacheKey')) return true;
          return paramMap.keys.some((key: string) => key.startsWith('filter['));
        }),
        takeUntilDestroyed(this.destroyRef),
        switchMap(async (paramMap: any) => {
          // Fast path: đọc từ sessionStorage
          const cacheKey = paramMap.get('cacheKey');
          if (cacheKey) {
            const cached = sessionStorage.getItem(cacheKey);
            if (cached) {
              sessionStorage.removeItem(cacheKey);
              return JSON.parse(cached);
            }
          }
          // Fallback: gọi API print riêng (không phân trang, đầy đủ data)
          const params: any = {};
          paramMap.keys.forEach((key: string) => {
            if (key !== 'cacheKey') params[key] = paramMap.get(key);
          });
          const response = await firstValueFrom(this.printService.receptionPayment(params));
          if (response?.object?.items) {
            response.object.items = await this.modelService.addLabels(response.object.items ?? []);
          }
          return response;
        }),
      )
      .subscribe({
        next: (response: any) => {
          this.totalAmount = response['totalAmount'] ?? 0;
          const data = response['object'] || response;
          if (data.items && Array.isArray(data.items)) {
            this.models.set(data.items);
            if (data.items.length > 0) {
              this.setupPrintLayout({
                branch_id: data.items[0].branch_id,
              });
            }
          } else {
            this.model.set(data);
            this.setupPrintLayout(data);
            this.models.set([data.payment]);
          }
          setTimeout(() => void this.handlePrintAfterInit());
        },
        error: (error) => console.error('Error loading payment:', error),
      });
  }

  private setupPrintLayout(model: any): void {
    this.printData.setModel({
      branch: model?.branch_id,
      customer: model?.customer || null,
      layoutType: 'with-accountant',
    });
  }
}
