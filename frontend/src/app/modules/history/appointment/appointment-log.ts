import {ChangeDetectionStrategy, Component, inject, OnInit, ViewChild } from '@angular/core';
import { Table } from 'primeng/table';
import { Observable, switchMap } from 'rxjs';

import { BaseListV2 } from '@/share/base/base-list-v2';
import { FilterFieldDef } from '@/share/base/list-filter.model';
import { ListApiResponse } from '@/share/models/list-api-response';
import { ListShareImports } from '@/share/import/list-share-imports';
import { LogsService } from '@/core/services/history/logs.service';
import { ActivatedRoute } from '@angular/router';
import { APPOINTMENT_BOOK_TYPE_MAP, APPOINTMENT_STATUS_MAP } from '@/define';
import { UserHelperService } from '@/indexeddb/config/components/user';
import { SourceHelperService } from '@/indexeddb/config/components/source';
import { ServicesHelperService } from '@/indexeddb/config/components/services';
import { BranchHelperService } from '@/indexeddb/config/components/branch';
import { ServiceTypeHelperService } from '@/indexeddb/config/components/service-type/service-type-helper.service';
import { AppointmentTypeHelperService } from '@/indexeddb/config/components/appointment-type';
import { FillHeightDirective } from '@/share/directive/fill-height.directive';
import { LOG_ACTION_SELECT } from '@/define';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-appointment-log',
  templateUrl: './appointment-log.html',
  standalone: true,
  imports: [FillHeightDirective, ...ListShareImports],
})
export class AppointmentLog extends BaseListV2<any> implements OnInit {
  modelService = inject(LogsService);
  appointmentTypeHelper = inject(AppointmentTypeHelperService);
  serviceTypeHelper = inject(ServiceTypeHelperService);
  branchHelper = inject(BranchHelperService);
  userHelper = inject(UserHelperService);
  sourceHelper = inject(SourceHelperService);
  servicesHelper = inject(ServicesHelperService);
  router = inject(ActivatedRoute);
  @ViewChild('dt') dt!: Table;
  protected override URL_STATE_PREFIX = 'la';
  /** Dòng xoá: không liệt kê snapshot `giá trị → null`, chỉ hiện 'Dữ liệu đã bị xóa'. */
  protected readonly LOG_ACTION_DELETE = LOG_ACTION_SELECT.LOG_ACTION_DELETE;
  private fieldLabels: Record<string, string> = {
    appointment_reason_id: 'Lý do huỷ hẹn',
    confirmAppointment: 'Xác nhận lịch hẹn',
    appointment_date: 'Ngày hẹn',
    checkin_date: 'Thời gian checkin',
    checkout_date: 'Thời gian checkout',
    status: 'Trạng thái',
    bookType: 'Phân loại',
    noteAppointment: 'Ghi chú',
    content: 'Nội dung lịch hẹn',
    revenue_id: 'Tư vấn viên',
    type_service_id: 'Nhóm dịch vụ quan tâm',
    campain_event_id: 'Chiến dịch',
    branch_id: 'Chi nhánh',
    appointment_type_id: 'Loại lịch hẹn',
    service_id: 'Dịch vụ hẹn',
    source_id: 'Nguồn khách hàng',
    // Cột marketing_id đã drop (2026-10-18) — GIỮ nhãn + format để đọc log lịch sử cũ còn key này.
    marketing_id: 'Marketing',
    doctor_id: 'Nhân viên tư vấn',
    doctorAppointment_id: 'Nhân viên hẹn',
  };

  constructor() {
    super();
  }

  ngOnInit() {
    this.requiredSearch.set({ paramUrl: this.router.snapshot.url.at(-1)?.path });
    this.initListV2();
  }

  protected filterFields(): FilterFieldDef[] {
    return [{ name: '_keyword', kind: 'text' }]; // → la_keyword
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
    if (
      (key === 'marketing_id' ||
        key === 'revenue_id' ||
        key === 'doctor_id' ||
        key === 'doctorAppointment_id') &&
      typeof value === 'number'
    ) {
      return this.userHelper.getLabel(value);
    }

    if (key === 'appointment_type_id' && typeof value === 'number') {
      return this.appointmentTypeHelper.getAppointmentTypeName(value);
    }
    if (key === 'branch_id' && typeof value === 'number') {
      return this.branchHelper.getBranchName(value);
    }
    if (key === 'type_service_id' && typeof value === 'number') {
      return this.serviceTypeHelper.getLabel(value);
    }
    if (key === 'service_id' && typeof value === 'number') {
      return this.servicesHelper.getServiceName(value);
    }
    if (key === 'source_id' && typeof value === 'number') {
      return this.sourceHelper.getSourceName(value);
    }
    if (
      (key === 'checkin_date' || key === 'checkout_date' || key === 'appointment_date') &&
      typeof value === 'number'
    ) {
      return this.formatTimestamp(value);
    }
    if (key === 'bookType' && typeof value === 'number') {
      return APPOINTMENT_BOOK_TYPE_MAP[value]?.label ?? value;
    }
    if (key === 'status' && typeof value === 'number') {
      return APPOINTMENT_STATUS_MAP[value]?.label ?? value;
    }
    if (key === 'confirmAppointment' && typeof value === 'number') {
      if (value == 1) {
        return 'Đã xác nhận';
      } else {
        return 'Chưa xác nhận';
      }
      return APPOINTMENT_BOOK_TYPE_MAP[value]?.label ?? value;
    }
    return value;
  }
}
