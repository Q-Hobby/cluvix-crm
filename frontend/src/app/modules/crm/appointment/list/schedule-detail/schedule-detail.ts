import { TranslatePipe } from '@ngx-translate/core';
import { ChangeDetectionStrategy, ChangeDetectorRef, Component, DestroyRef, inject } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { DetailShareImports } from '@/share/import/detail-share-imports';
import { BaseDetailDialog } from '@/share/base/base-detail-dialog';
import { AppointmentService } from '@/core/services/crm/appointment.service';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-schedule-detail-detail',
  templateUrl: './schedule-detail.html',
  styles: [
    `
      ::ng-deep .p-datatable .p-datatable-thead > tr > th {
        background: #dbeafe !important;
        border-bottom: 1px solid #93c5fd !important;
      }
      ::ng-deep .p-datatable[class*='dark'] .p-datatable-thead > tr > th {
        background: rgba(30, 58, 138, 0.3) !important;
        border-bottom: 1px solid rgba(59, 130, 246, 0.7) !important;
      }
    `,
  ],
  standalone: true,
  imports: [...DetailShareImports, TranslatePipe],
})
export class ScheduleDetail extends BaseDetailDialog<any> {
  modelService = inject(AppointmentService);
  private cdr = inject(ChangeDetectorRef);
  private destroyRef = inject(DestroyRef);
  timeSlot: string;
  displayName: string;

  // Table properties
  appointments: any[] = [];
  totalRecords = 0;
  readonly pageSize = 15;
  first = 0;

  constructor() {
    super();
    this.timeSlot = this.dialogConfig?.data?.timeSlot;
    this.displayName = this.dialogConfig?.data?.branch;
  }

  loadAppointments(event: any) {
    const { timeSlot, branchId, typeId, appointmentDate } = this.dialogConfig.data;
    if (!timeSlot || !branchId || !typeId) return;

    // Handle different timeSlot formats
    let startTimestamp: number;
    let endTimestamp: number;

    if (timeSlot.includes('–')) {
      // Hour format: "09:00–10:00" — lt: endTime excludes appointments at slot boundary
      const [startTime, endTime] = timeSlot.split('–');
      const selectedDate = appointmentDate ?? this.timezoneService.now();
      startTimestamp = this.toUnixTimestamp(selectedDate, startTime);
      endTimestamp = this.toUnixTimestamp(selectedDate, endTime);
    } else {
      // Day format: "10-10-2025" — +1 to reach midnight next day (include full day)
      const selectedDate = appointmentDate;
      startTimestamp = this.toUnixTimestamp(selectedDate, '00:00:00');
      endTimestamp = this.toUnixTimestamp(selectedDate, '23:59:59') + 1;
    }

    // Use BaseList logic for pagination
    const { first, rows } = event;
    const page = Math.floor(first / rows) + 1;
    const params: any = {
      'filter[appointment_type_id]': typeId,
      'filter[appointment_date][gte]': startTimestamp,
      'filter[appointment_date][lt]': endTimestamp,
      sort: 'appointment_date',
      page,
      'per-page': rows,
    };

    // Add specific filter based on schedule type
    if (branchId) {
      params['filter[branch_id]'] = branchId;
    }

    this.loading.set(true);
    this.modelService
      .list({ params })
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe({
        next: async (response: any) => {
          try {
            const items = await this.modelService.addLabels(response.items ?? []);

            this.appointments = items;
            this.totalRecords = response?._meta?.totalCount ?? 0;
            this.first = event.first;
          } catch (err) {
            console.error('[ScheduleDetail] async subscribe error:', err);
            this.noti.errorMsg(this.translate.instant('common.message.error'));
          } finally {
            this.loading.set(false);
            this.cdr.markForCheck();
          }
        },
        error: (err: any) => {
          console.error('[ScheduleDetail] list error:', err);
          this.loading.set(false);
          this.noti.errorMsg(err?.message || this.translate.instant('common.message.error'));
          this.cdr.markForCheck();
        },
      });
  }

  private toUnixTimestamp(date: Date, time: string): number {
    const [h, m, s] = time.split(':').map((p) => parseInt(p) || 0);
    return this.timezoneService.dateHmsToUnix(date, h, m, s);
  }
}
