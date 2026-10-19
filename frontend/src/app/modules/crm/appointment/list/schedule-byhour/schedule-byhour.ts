import {ChangeDetectionStrategy, Component, inject, OnInit, signal} from '@angular/core';
import {FormGroup, ReactiveFormsModule} from '@angular/forms';
import {ButtonModule} from 'primeng/button';
import {DatePickerModule} from 'primeng/datepicker';
import {FloatLabelModule} from 'primeng/floatlabel';
import {TooltipModule} from 'primeng/tooltip';
import {ToolbarModule} from 'primeng/toolbar';
import {finalize} from 'rxjs';
import {TranslatePipe} from '@ngx-translate/core';

import {PERMISSION_SELECT, STORAGE_KEY_SELECT} from '@/define';
import {HasPermDirective} from '@/share/directives/has-perm.directive';
import {AppointmentService} from '@/core/services/crm/appointment.service';
import {BaseApp} from '@/share/base/BaseApp';
import {ScheduleByhourList} from './schedule-byhour-list';
import {ScheduleByhourView} from './schedule-byhour-view';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-schedule-byhour',
  standalone: true,
  imports: [
    ReactiveFormsModule,
    ButtonModule,
    DatePickerModule,
    FloatLabelModule,
    TooltipModule,
    ToolbarModule,
    ScheduleByhourList,
    ScheduleByhourView,
    TranslatePipe,
    HasPermDirective,
  ],
  styles: [
    `
      :host ::ng-deep .p-toolbar {
        background: white !important;
      }
    `,
  ],
  template: `
    @if (rfFilterModel) {
      <p-toolbar>
        <ng-template #start>
          <div class="flex gap-1">
            <button type="button"
              pButton
              (click)="setViewMode('matrix')"
              [severity]="viewMode() === 'matrix' ? 'primary' : 'secondary'"
              [pTooltip]="'crm.appointment.tooltip.view_matrix' | translate"
              [iconOnly]="true"
            ><i class="pi pi-th-large"></i></button>
            <button type="button"
              pButton
              (click)="setViewMode('detail')"
              [severity]="viewMode() === 'detail' ? 'primary' : 'secondary'"
              [pTooltip]="'crm.appointment.tooltip.view_detail' | translate"
              [iconOnly]="true"
            ><i class="pi pi-list"></i></button>
            <button type="button"
              pButton
              (click)="printList()"
              [pTooltip]="'crm.appointment.tooltip.print_list' | translate"
              severity="info"
              [iconOnly]="true"
            ><i class="pi pi-print"></i></button>
            <!-- BE: GET /print/appointment-byhour-list-export đòi extAppointment (RequirePermStrict) -->
            <button type="button"
              *hasPerm="permission.permissionExtAppointment"
              pButton
              (click)="exportScheduleByhour($event)"
              [pTooltip]="'common.tooltip.export_data' | translate"
              severity="help"
              tooltipPosition="top"
              [iconOnly]="true"
            ><i class="pi pi-download"></i></button>
          </div>
        </ng-template>
        <ng-template #end>
          <form [formGroup]="rfFilterModel" novalidate>
            <div class="flex items-center gap-2">
              <p-floatlabel variant="on">
                <p-datepicker
                  [readonlyInput]="true"
                  appendTo="body"
                  dateFormat="dd/mm/yy"
                  formControlName="appointment_date"
                  showIcon="true"
                ></p-datepicker>
                <label for="appointment_date">{{ 'common.label.appointment_date' | translate }}</label>
              </p-floatlabel>
              <button type="button"
                pButton
                (click)="resetPage()"
                [pTooltip]="'common.action.reload_page' | translate"
                severity="secondary"
                tooltipPosition="top"
                [iconOnly]="true"
              ><i class="pi pi-refresh"></i></button>
            </div>
          </form>
        </ng-template>
      </p-toolbar>
    }

    @if (viewMode() === 'matrix') {
      <app-schedule-byhour-view [appointmentDate]="rfFilterModel.value.appointment_date"/>
    } @else {
      <app-schedule-byhour-list [appointmentDate]="rfFilterModel.value.appointment_date"/>
    }
  `,
})
export class ScheduleByhour extends BaseApp implements OnInit {
  private appointmentService = inject(AppointmentService);
  protected readonly permission = PERMISSION_SELECT;
  viewMode = signal<'matrix' | 'detail'>('matrix');
  rfFilterModel: FormGroup;

  constructor() {
    super();
    this.rfFilterModel = this.fb.group({
      appointment_date: [this.timezoneService.now()],
    });
  }

  ngOnInit() {
    const stored = this.storageService.loadStorage(STORAGE_KEY_SELECT.ScheduleViewMode, false);
    this.viewMode.set(stored === 'detail' ? 'detail' : 'matrix');
  }

  setViewMode(mode: 'matrix' | 'detail') {
    this.viewMode.set(mode);
    this.storageService.saveStorage(STORAGE_KEY_SELECT.ScheduleViewMode, mode, true);
  }

  resetPage() {
    this.rfFilterModel.patchValue({ appointment_date: this.timezoneService.now() });
  }

  printList() {
    const d = this.rfFilterModel.value.appointment_date;
    if (!d) return;
    const dateStr = this.toYmd(d);
    this.openPrint(
      `/print/print-appointment-byhour-list?appointment_date=${dateStr}`,
      `print-appt-byhour-${dateStr}`,
      this.appointmentService.printAppointmentByhourList({ params: { appointment_date: dateStr } }),
      (data: any) => Array.isArray(data) ? data.length > 0 : data?.data?.length > 0,
    );
  }

  exportScheduleByhour(event: Event) {
    if (!this.permissionsService.getPermission(PERMISSION_SELECT.permissionExtAppointment)) {
      this.noti.warnMsg(this.translate.instant('common.message.no_permission'));
      return;
    }
    const d = this.rfFilterModel.value.appointment_date;
    if (!d) return;
    const dateStr = this.toYmd(d);
    this.confirmationService.confirm({
      target: event.target ?? undefined,
      message: this.translate.instant('common.message.confirm_export'),
      icon: 'pi pi-exclamation-triangle',
      acceptButtonProps: {severity: 'danger', size: 'small'},
      accept: () => {
        this.loadingService.showLoading('export-byhour');
        this.appointmentService.exportAppointmentByhourList({ params: { appointment_date: dateStr } })
          .pipe(finalize(() => this.loadingService.hideLoading('export-byhour')))
          .subscribe({
            next: (url: string) => {
              this.noti.successMsg(this.translate.instant('common.message.export_success'));
              this.downloadExport(url);
            },
          });
      },
      reject: () => this.isSubmitting.set(false),
    });
  }

  private toYmd(d: Date): string {
    const y = d.getFullYear();
    const m = String(d.getMonth() + 1).padStart(2, '0');
    const day = String(d.getDate()).padStart(2, '0');
    return `${y}-${m}-${day}`;
  }
}
