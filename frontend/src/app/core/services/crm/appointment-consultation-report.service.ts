import { inject, Service } from '@angular/core';

import {
  AppointmentConsultationDisplayModel,
} from '@/core/models/crm/appointment-consultation-report.model';
import { AppointmentModel } from '@/core/models/crm/appointment.model';
import { AppointmentTypeHelperService } from '@/indexeddb/config/components/appointment-type';
import { AppointmentReasonHelperService } from '@/indexeddb/config/components/appointment-reason';
import { BranchHelperService } from '@/indexeddb/config/components/branch';
import {
  ServiceTypeHelperService,
} from '@/indexeddb/config/components/service-type/service-type-helper.service';
import { UserHelperService } from '@/indexeddb/config/components/user';
import { BaseApiService } from '@/share/services/base-api.service';

/** Report "Lịch hẹn tư vấn" (CRM) — GET /crm/report/appointment-consultation.
 * BE chỉ trả raw ID cho các field tham chiếu (bác sĩ, chi nhánh, loại hẹn, nhóm
 * dịch vụ, lý do từ chối, người tạo) — FE resolve tên/avatar qua IndexedDB cache
 * trong `addLabel()`, KHÔNG gọi thêm API nào (xem architecture.md FR7). */
@Service()
export class AppointmentConsultationReportService extends BaseApiService<AppointmentModel> {
  protected override endpoint = 'report/appointment-consultation';
  protected override unixTimesTampFields = [
    'appointment_date',
    'checkin_date',
    'checkout_date',
    'created_at',
  ];

  private branchHelper = inject(BranchHelperService);
  private appointmentTypeHelper = inject(AppointmentTypeHelperService);
  private serviceTypeHelper = inject(ServiceTypeHelperService);
  private appointmentReasonHelper = inject(AppointmentReasonHelperService);
  private userHelper = inject(UserHelperService);

  exportAppointmentConsultation(params: any = {}) {
    return this.apiService.get<string>(`${this.path}?_export=1`, params);
  }

  addLabel(item: AppointmentModel): AppointmentConsultationDisplayModel {
    const itemWithDates = this.addFormattedTimestampFields(item, [
      'appointment_date',
      'checkin_date',
      'checkout_date',
      'created_at',
    ]);

    return {
      ...itemWithDates,
      branchLabel: this.branchHelper.getBranchName(item.branch_id),
      appointmentTypeLabel: this.appointmentTypeHelper.getAppointmentTypeName(item.appointment_type_id),
      serviceTypeAcceptedLabel: this.serviceTypeHelper.getLabel(item.type_service_accepted_id),
      appointmentReasonLabel: this.appointmentReasonHelper.getAppointmentReasonInfo(item.appointment_reason_id!)?.name ?? '',
      createdByInfo: this.userHelper.getUserInfo(item.created_by),
    };
  }

  async addLabels(items: AppointmentModel[]): Promise<AppointmentConsultationDisplayModel[]> {
    if (!Array.isArray(items)) return [];
    await this.ensureCacheLoaded();
    return items.map((item) => this.addLabel(item));
  }

  async ensureCacheLoaded() {
    await Promise.all([
      this.branchHelper.ensureBranchesLoaded(),
      this.appointmentTypeHelper.ensureAppointmentTypesLoaded(),
      this.serviceTypeHelper.ensureServiceTypesLoaded(),
      this.appointmentReasonHelper.ensureAppointmentReasonLoaded(),
      this.userHelper.ensureUsersLoaded(),
    ]);
  }
}
