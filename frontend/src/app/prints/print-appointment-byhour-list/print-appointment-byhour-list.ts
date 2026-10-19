import { ChangeDetectionStrategy, Component, DestroyRef, inject, OnInit, signal, ViewEncapsulation } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { UserHelperService } from '@/indexeddb/config/components/user/user-helper.service';
import { APPOINTMENT_STATUS_MAP } from '@/define/appointment.define';
import { BasePrint } from '@/share/base/base-print';
import { PrintShareImport } from '@/share/import/print-share-import';
import { finalize } from 'rxjs';
import { PrintService } from '@/prints/print.service';
import { SanitizeQuillPipe } from '@/share/pipes/sanitize-quill.pipe';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: 'print-appointment-byhour-list.html',
  styleUrls: ['print-appointment-byhour-list.scss'],
  encapsulation: ViewEncapsulation.None,
  imports: [...PrintShareImport, SanitizeQuillPipe],
  standalone: true,
})
export class PrintAppointmentByhourList extends BasePrint implements OnInit {
  private printService = inject(PrintService);
  groupedData = signal<any[]>([]);
  appointmentDate = signal<string>('');
  private userHelper = inject(UserHelperService);
  private destroyRef = inject(DestroyRef);

  ngOnInit() {
    this.activatedRoute.queryParams.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((queryParams: any) => {
      // Thử đọc từ sessionStorage trước
      const cacheKey = queryParams['cacheKey'];
      const cached = this.tryLoadFromCache(cacheKey);
      if (cached) {
        this.applyData(cached);
        return;
      }

      // Fallback: gọi API print riêng từ module print
      const appointmentDate = queryParams.appointment_date || new Date().toISOString().split('T')[0];
      this.appointmentDate.set(appointmentDate);

      this.userHelper.ensureUsersLoaded().then(() => {
        this.printService
          .appointmentByhourList({ params: { appointment_date: appointmentDate } })
          .pipe(finalize(() => {}))
          .subscribe((response: any) => this.applyData(response));
      });
    });
  }

  applyData(response: any) {
    const appointments = response?.data || response || [];
    // Process appointments
    appointments.forEach((appointment: any) => {
      const statusNum = Number(appointment.status);
      appointment.statusInfo = APPOINTMENT_STATUS_MAP[statusNum];
    });

    // Lấy appointment_date từ data nếu có
    if (appointments.length > 0 && appointments[0].appointment_date) {
      const timestamp = appointments[0].appointment_date;
      const date = new Date(timestamp * 1000);
      this.appointmentDate.set(date.toISOString());
    }

    // Group by branch
    const grouped = this.groupByBranch(appointments);
    this.groupedData.set(grouped);
    this.printData.setModel({
      layoutType: 'simple',
    });
    setTimeout(() => void this.handlePrintAfterInit());
  }

  getTotalAppointments(): number {
    return this.groupedData().reduce((total, group) => total + group.appointments.length, 0);
  }

  getCurrentDate(): Date {
    return new Date();
  }

  private groupByBranch(appointments: any[]) {
    const groups = new Map<number, any>();

    appointments.forEach((appointment) => {
      if (!groups.has(appointment.branch_id)) {
        groups.set(appointment.branch_id, {
          branch_id: appointment.branch_id,
          branch_name: appointment.branch_name,
          appointments: [],
        });
      }
      groups.get(appointment.branch_id)!.appointments.push(appointment);
    });

    return Array.from(groups.values()).sort((a, b) => a.branch_name.localeCompare(b.branch_name));
  }
}
