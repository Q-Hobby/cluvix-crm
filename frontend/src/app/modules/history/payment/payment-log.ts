import {ChangeDetectionStrategy, Component, effect, inject, OnInit, ViewChild } from '@angular/core';
import { DecimalPipe } from '@angular/common';
import { Table } from 'primeng/table';
import { Observable, switchMap } from 'rxjs';

import { BaseListV2 } from '@/share/base/base-list-v2';
import { FilterFieldDef } from '@/share/base/list-filter.model';
import { ListApiResponse } from '@/share/models/list-api-response';
import { ListShareImports } from '@/share/import/list-share-imports';
import { LogsService } from '@/core/services/history/logs.service';
import { ActivatedRoute } from '@angular/router';
import { BranchHelperService } from '@/indexeddb/config/components/branch';
import { PaymentMethodHelperService } from '@/indexeddb/config/components/payment-method';
import { LocaleService } from '@/share/services/locale.service';
import { PaymentTypeHelperService } from '@/indexeddb/config/components/payment-type';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';
import { LOG_ACTION_SELECT } from '@/define';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-payment-log',
  templateUrl: './payment-log.html',
  standalone: true,
  imports: [FillHeightDirective, ...ListShareImports],
})
export class PaymentLog extends BaseListV2<any> implements OnInit {
  modelService = inject(LogsService);
  router = inject(ActivatedRoute);
  @ViewChild('dt') dt!: Table;
  protected override URL_STATE_PREFIX = 'lp';
  /** Dòng xoá: không liệt kê snapshot `giá trị → null`, chỉ hiện 'Dữ liệu đã bị xóa'. */
  protected readonly LOG_ACTION_DELETE = LOG_ACTION_SELECT.LOG_ACTION_DELETE;
  private branchHelper = inject(BranchHelperService);
  private paymentTypeHelper = inject(PaymentTypeHelperService);
  private paymentMethodHelper = inject(PaymentMethodHelperService);
  private localeService = inject(LocaleService);
  private fieldLabels: Record<string, string> = {
    branch_id: 'Chi nhánh',
    method: 'Phương thức thanh toán',
    card_type_id: 'Hình thức thanh toán',
    bank_code: 'Hoá đơn',
    amount: 'Số tiền',
    payment_time: 'Ngày thu tiền',
    payment_of_date: 'Ngày thanh toán',
    note: 'Ghi chú',
  };

  constructor() {
    super();
    effect(() => {
      this.padCols.set(7);
    });
  }

  ngOnInit() {
    this.requiredSearch.set({ paramUrl: this.router.snapshot.url.at(-1)?.path });
    this.initListV2();
  }

  protected filterFields(): FilterFieldDef[] {
    return [{ name: '_keyword', kind: 'text' }]; // → lp_keyword
  }

  protected override fetchList(): Observable<ListApiResponse<any>> {
    return this.modelService.listLogs({ params: this.searchParams() }).pipe(
      switchMap(async (response: any) => {
        if (response.items) {
          const items = response.items ?? [];
          response.items = await this.modelService.addLabels(items);
          response.items.forEach((item: any) => {
            item.changed_fields = item.changed_fields.map((field: any) => ({
              key: this.fieldLabels[field.key] || field.key,
              value: {
                from: this.formatFieldValue(field.key, field.value?.from),
                to: this.formatFieldValue(field.key, field.value?.to),
              },
            }));
          });
        }
        return response;
      }),
    );
  }

  /**
   * D-13: bản cũ dùng new Date(timestamp*1000).getHours()/getDate() — GIỜ MÁY trình duyệt, lệch
   * với bộ lọc ngày (đã chuyển sang giờ công ty ở §4.4). Đổi sang cùng kỹ thuật với
   * TimezoneService.toTz(): Intl.DateTimeFormat theo tz() công ty, giữ nguyên format hiển thị.
   */
  private formatTimestamp(timestamp: number): string {
    const formatter = new Intl.DateTimeFormat('en-US', {
      timeZone: this.timezoneService.tz(),
      year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false,
    });
    const parts = formatter.formatToParts(new Date(timestamp * 1000));
    const get = (type: string) => parts.find((p) => p.type === type)?.value ?? '00';
    return `${get('day')}-${get('month')}-${get('year')} ${get('hour')}:${get('minute')}`;
  }

  private formatFieldValue(key: string, value: any): any {
    if ((key === 'payment_time' || key === 'payment_of_date') && typeof value === 'number') {
      return this.formatTimestamp(value);
    }
    if (key === 'branch_id' && typeof value === 'number') {
      return this.branchHelper.getBranchName(value);
    }
    if (key === 'method' && typeof value === 'number') {
      return this.paymentMethodHelper.getPaymentMethodName(value);
    }
    if (key === 'card_type_id' && typeof value === 'number') {
      return this.paymentTypeHelper.getPaymentTypeName(value);
    }
    if (key === 'amount' && value !== '-' && value !== null && !isNaN(value)) {
      const decimalPipe = new DecimalPipe(this.localeService.getLocale());
      return decimalPipe.transform(parseInt(value), '1.0');
    }

    return value;
  }
}
