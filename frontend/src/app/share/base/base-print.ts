import { inject, Service, signal } from '@angular/core';
import { Title } from '@angular/platform-browser';
import { ActivatedRoute } from '@angular/router';
import { TranslateService } from '@ngx-translate/core';

import { PrintDataService } from '@/layout/print/print-data.service';
import { TimezoneService } from '@/core/services/timezone.service';

@Service()
export abstract class BasePrint {
  title = inject(Title); // Optional nếu bạn muốn cập nhật title browser cho trang chi tiết.
  readonly model = signal<any>({});
  readonly models = signal<any[]>([]);
  private timezoneService = inject(TimezoneService);
  readonly today = this.timezoneService.now();
  readonly formattedDate = signal<string>('');
  readonly orderData = signal<{ date: number; month: number; year: number }>({
    date: 0,
    month: 0,
    year: 0,
  });
  protected printData = inject(PrintDataService);
  protected readonly activatedRoute = inject(ActivatedRoute);
  protected readonly translate = inject(TranslateService);

  constructor() {
    this.title.setTitle('Print');
  }

  async formatDate(date: Date): Promise<void> {
    this.orderData.set({
      date: date.getDate(),
      month: date.getMonth() + 1,
      year: date.getFullYear(),
    });
  }

  protected async handlePrintAfterInit(): Promise<void> {
    await this.formatDate(new Date());
  }

  // Đọc data từ sessionStorage (do parent đã fetch sẵn), xóa sau khi đọc để tránh leak
  protected tryLoadFromCache(cacheKey: string | null | undefined): any | null {
    if (!cacheKey) return null;
    const cached = sessionStorage.getItem(cacheKey);
    if (cached) {
      sessionStorage.removeItem(cacheKey);
      return JSON.parse(cached);
    }
    return null;
  }
}
