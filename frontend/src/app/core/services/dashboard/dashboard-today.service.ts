import { Service} from '@angular/core';
import { Observable } from 'rxjs';

import { BaseApiService } from '@/share/services/base-api.service';
import { PAYMENT_FOR_MAP } from '@/define/payment.define';

@Service()
export class DashboardTodayService extends BaseApiService<any> {
  getRevenueToday(params: any = {}): Observable<any> {
    return this.apiService.get(`crm/order-service-used/revenue-today`, params);
  }

  getReceiptsToday(params: any = {}): Observable<any> {
    return this.apiService.get(`crm/payment/receipts-today`, params);
  }

  addLabel(item: any) {
    return {
      ...item,
      paymentForLabel: PAYMENT_FOR_MAP[item.payment_for] || 'Không xác định',
      paymentForClass: `paymentFor-${item.payment_for}`,
    };
  }

  async addLabels(items: any[]) {
    if (!Array.isArray(items)) return [];
    return items.map((item) => this.addLabel(item));
  }
}
