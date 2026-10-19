import { ChangeDetectionStrategy, Component, inject, OnInit, ViewChild } from '@angular/core';
import { Table } from 'primeng/table';
import { Observable } from 'rxjs';

import { BaseListV2 } from '@/share/base/base-list-v2';
import { FilterFieldDef } from '@/share/base/list-filter.model';
import { ListApiResponse } from '@/share/models/list-api-response';
import { ListShareImports } from '@/share/import/list-share-imports';
import { AuthService } from '@/core/services/auth/auth.service';
import { UserFormControl } from '@/indexeddb/config/components/user';
import { UserName } from '@/indexeddb/config/components/user/user-name';
import { DatePickerFooter } from '@/share/components/date-picker-footer';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';

// Chỉ nhóm event liên quan phiên đăng nhập (BE lọc qua ?events= khi không chọn 1 event cụ thể).
const LOGIN_EVENTS = ['login_success', 'login_failed', 'login_no_fingerprint', 'new_device', 'logout', '2fa_challenge', '2fa_failed'];

/**
 * Lịch sử đăng nhập — super-user kiểm soát login/logout/thiết bị mới của company active.
 * Đọc từ security_audit_log (nhóm login events) qua GET /auth/audit-log.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-login-history',
  templateUrl: './login-history.html',
  standalone: true,
  imports: [FillHeightDirective, ...ListShareImports, UserFormControl, UserName, DatePickerFooter],
})
export class LoginHistory extends BaseListV2<any> implements OnInit {
  modelService = inject(AuthService);
  @ViewChild('dt') dt!: Table;
  protected override URL_STATE_PREFIX = 'lh';

  readonly eventOptions = LOGIN_EVENTS.map((e) => ({
    label: this.translate.instant('history.login_history.event_' + e),
    value: e,
  }));

  ngOnInit() {
    this.initListV2();
  }

  /**
   * Khai báo filter — base tự sinh FormGroup, đấu valueChanges → dt.filter(),
   * dựng filters khởi tạo cho p-table, lo clearFilter() và đồng bộ URL.
   * Giữ nguyên tên control `dateRange` (khác 5 list kia): def.name ở đây không phải tên
   * field gửi BE (BE dùng from/to), nên giữ tên cũ đỡ phải sửa thêm 3 chỗ.
   */
  protected filterFields(): FilterFieldDef[] {
    return [
      { name: 'event', kind: 'equals', itemType: 'string' }, // → lh_event
      { name: 'user_id', kind: 'equals' }, // → lh_user_id (số)
      { name: 'dateRange', kind: 'dateRange', urlKey: 'date' }, // → lh_date
    ];
  }

  clearDateRange() {
    this.setFilterValue('dateRange', null);
  }

  onQuickSelectDateRange({ start, end }: { start: Date; end: Date }) {
    this.rfFilterModel.patchValue({ dateRange: [start, end] });
  }

  // Endpoint /auth/audit-log dùng query PHẲNG (event/events/user_id/from/to), không theo
  // convention filter[field][matchMode] → cố ý không dùng searchParams()/search() của base.
  protected override fetchList(): Observable<ListApiResponse<any>> {
    const { event, user_id, dateRange } = this.rfFilterModel.getRawValue();
    const params: Record<string, any> = {
      page: this.page(),
      'per-page': this.perPage(),
    };
    if (event) params['event'] = event;
    else params['events'] = LOGIN_EVENTS.join(',');
    if (user_id) params['user_id'] = user_id;
    const range = this.timezoneService.buildFilterTimestamp(dateRange); // [gte, lte] Unix GIÂY
    if (range) {
      params['from'] = range[0];
      params['to'] = range[1];
    }
    return this.modelService.getAuditLog(params);
  }

  private detailObj(row: any): any {
    if (!row?.detail) return {};
    try {
      return typeof row.detail === 'string' ? JSON.parse(row.detail) : row.detail;
    } catch {
      return {};
    }
  }

  reasonOf(row: any): string {
    return this.detailObj(row)['reason'] ?? '';
  }

  deviceOf(row: any): string {
    return this.detailObj(row)['device'] ?? '';
  }

  eventSeverity(event: string): 'success' | 'warn' | 'danger' | 'info' {
    if (event === 'login_success' || event === 'new_device') return 'success';
    if (event === 'logout') return 'info';
    if (event === 'login_failed' || event === 'login_no_fingerprint') return 'warn';
    return 'info';
  }
}
