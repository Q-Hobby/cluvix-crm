import {ChangeDetectionStrategy, Component, effect, inject, OnInit, ViewChild } from '@angular/core';
import { Table } from 'primeng/table';
import { Observable, switchMap } from 'rxjs';

import { BaseListV2 } from '@/share/base/base-list-v2';
import { FilterFieldDef } from '@/share/base/list-filter.model';
import { ListApiResponse } from '@/share/models/list-api-response';
import { ListShareImports } from '@/share/import/list-share-imports';
import { LogsService } from '@/core/services/history/logs.service';
import { ActivatedRoute } from '@angular/router';
import { ServiceTypeHelperService } from '@/indexeddb/config/components/service-type/service-type-helper.service';
import { BranchHelperService } from '@/indexeddb/config/components/branch';
import { UserHelperService } from '@/indexeddb/config/components/user';
import { SourceHelperService } from '@/indexeddb/config/components/source';
import { CustomerCategoryHelperService } from '@/indexeddb/config/components/customer-category';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';
import { LOG_ACTION_SELECT } from '@/define';

/** Key `changed_fields` BE ghi khi xoá file đính kèm của khách (crm/attachment delete.go attachmentLogField). */
const ATTACHMENT_LOG_FIELD = 'attachment';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-customer-log',
  templateUrl: './customer-log.html',
  standalone: true,
  imports: [FillHeightDirective, ...ListShareImports],
})
export class CustomerLog extends BaseListV2<any> implements OnInit {
  modelService = inject(LogsService);
  router = inject(ActivatedRoute);
  @ViewChild('dt') dt!: Table;
  protected override URL_STATE_PREFIX = 'lc';
  /** Dòng xoá: không liệt kê snapshot `giá trị → null`, chỉ hiện 'Dữ liệu đã bị xóa'. */
  protected readonly LOG_ACTION_DELETE = LOG_ACTION_SELECT.LOG_ACTION_DELETE;
  private serviceTypeHelper = inject(ServiceTypeHelperService);
  private branchHelper = inject(BranchHelperService);
  private userHelper = inject(UserHelperService);
  private sourceHelper = inject(SourceHelperService);
  private customerCategoryHelper = inject(CustomerCategoryHelperService);
  private fieldLabels: Record<string, string> = {
    code: 'Mã khách hàng',
    fullname: 'Tên khách hàng',
    nickname: 'Tên khác khách hàng',
    phone: 'Số điện thoại',
    category: 'Nhóm khách hàng',
    branch_id: 'Chi nhánh',
    // Cột marketing_id đã drop (2026-10-18) — GIỮ nhãn + format để đọc log lịch sử cũ còn key này.
    marketing_id: 'Marketing',
    payment_user_id: 'Tư vấn viên',
    type_service_id: 'Nhóm dịch vụ quan tâm',
    source_id: 'Nguồn khách hàng',
    created_day: 'Ngày thu thập',
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
    return [{ name: '_keyword', kind: 'text' }]; // → lc_keyword
  }

  protected override fetchList(): Observable<ListApiResponse<any>> {
    return this.modelService.listLogs({ params: this.searchParams() }).pipe(
      switchMap(async (response: any) => {
        if (response.items) {
          const items = response.items ?? [];
          response.items = await this.modelService.addLabels(items);
          response.items.forEach((item: any) => {
            item.changed_fields = item.changed_fields.map((field: any) => ({
              key: field.key === ATTACHMENT_LOG_FIELD
                ? this.translate.instant('crm.customer_attachment.history_label')
                : this.fieldLabels[field.key] || field.key,
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
    if ((key === 'marketing_id' || key === 'payment_user_id') && typeof value === 'number') {
      return this.userHelper.getLabel(value);
    }

    if (key === 'category' && typeof value === 'number') {
      return this.customerCategoryHelper.getCustomerCategoryName(value);
    }
    if (key === 'source_id' && typeof value === 'number') {
      return this.sourceHelper.getSourceName(value);
    }
    if (key === 'created_day' && typeof value === 'number') {
      return this.formatTimestamp(value);
    }
    if (key === 'branch_id' && typeof value === 'number') {
      return this.branchHelper.getBranchName(value);
    }
    if (key === 'type_service_id' && typeof value === 'number') {
      return this.serviceTypeHelper.getLabel(value);
    }
    return value;
  }
}
