import {HttpParams} from '@angular/common/http';
import {Service} from '@angular/core';
import {Observable} from 'rxjs';

import {BaseApiService} from '@/share/services/base-api.service';

/** 1 dòng phẳng của báo cáo Hình thức thanh toán (hình thức × loại phiếu → tổng tiền). */
export interface PaymentByMethodRow {
  method_id: number;
  method_name: string;
  payment_for: number;
  amount: number;
}

/**
 * API nhóm báo cáo DOANH THU của CRM — BE `GET /crm/report/revenue/*`
 * (`backend/internal/modules/webapp/crm/report/revenue`). Mọi endpoint BẮT BUỘC dải ngày, trần 1 năm
 * (thiếu/vượt ⇒ 422).
 */
@Service()
export class ReportRevenueCrmService extends BaseApiService<any> {
  protected override apiModule = 'crm';
  protected override endpoint = 'report/revenue';

  /** Doanh thu theo ngày thu tiền — `{items, _meta, summary}`; lọc `_ngay_thang[gte|lte]`, `branch_id`. */
  getPaymentByDate(params: any = {}): Observable<any> {
    return this.apiService.get(`${this.path}/by-date`, params);
  }

  /** Export Excel doanh thu theo ngày — trả URL tải file. */
  exportPaymentByDate(params: any = {}): Observable<string> {
    return this.apiService.get(`${this.path}/by-date/export`, params);
  }

  /** Hình thức thanh toán — flat rows, FE pivot. `from`/`to` = unix giây. */
  getPaymentByMethod(from: number, to: number): Observable<PaymentByMethodRow[]> {
    const params = new HttpParams().set('from', from).set('to', to);
    return this.apiService.get<PaymentByMethodRow[]>(`${this.path}/by-method`, {params});
  }

  /** Phiếu thu theo chi nhánh — `{object: {items, _meta}, totalSum}`. */
  listBranchReceipts(params: any = {}): Observable<any> {
    return this.apiService.get(`${this.path}/branch-receipts`, params);
  }

  /** Export phiếu thu theo chi nhánh (`_export=1`, chọn cột qua `columns`) — trả URL tải file. */
  exportBranchReceipts(params: any = {}): Observable<string> {
    const query = {...(params?.params ?? params), _export: 1};
    return this.apiService.get(`${this.path}/branch-receipts`, {params: query});
  }

  /** Giảm trừ doanh số (hoàn dịch vụ + hoàn cọc) — `{object: {items, _meta}, totalRefundDeposit}`. */
  listRefundDepositService(params: any = {}): Observable<any> {
    return this.apiService.get(`${this.path}/refund-deposit-service`, params);
  }

  /** Export giảm trừ doanh số (`_export=1`, chọn cột qua `columns`) — trả URL tải file. */
  exportRefundDepositService(params: any = {}): Observable<string> {
    const query = {...(params?.params ?? params), _export: 1};
    return this.apiService.get(`${this.path}/refund-deposit-service`, {params: query});
  }
}
