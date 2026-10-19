import { ChangeDetectionStrategy, Component, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { TranslatePipe } from '@ngx-translate/core';
import { TableModule, TableLazyLoadEvent } from 'primeng/table';
import { SelectModule } from 'primeng/select';
import { InputTextModule } from 'primeng/inputtext';
import { DatePickerModule } from 'primeng/datepicker';
import { ButtonModule } from 'primeng/button';
import { TagModule } from 'primeng/tag';
import { finalize } from 'rxjs';

import { BaseApp } from '@/share/base/BaseApp';
import { UserName } from '@/indexeddb/config/components/user/user-name';

interface AuditRow {
  id: number;
  event: string;
  user_id: number | null;
  username: string | null;
  ip: string | null;
  request_id: string | null;
  detail: any;
  created_at: number;
}

// Danh sách event để filter — khớp pkg/seclog.
const EVENTS = [
  'login_success',
  'login_failed',
  'login_no_fingerprint',
  'new_device',
  'refresh_fp_mismatch',
  'access_ip_mismatch',
  'device_evicted',
  'device_revoked',
  'revoked_token_reused',
  'ip_not_allowed',
];

/**
 * Trang "Nhật ký bảo mật" — super-user xem security events của company active.
 * Server-paginated (p-table lazy), filter theo event/user/ngày. Read-only.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  standalone: true,
  imports: [
    CommonModule,
    FormsModule,
    TranslatePipe,
    TableModule,
    SelectModule,
    InputTextModule,
    DatePickerModule,
    ButtonModule,
    TagModule,
    UserName,
  ],
  template: `
    <div class="card p-4">
      <h2 class="text-xl font-semibold mb-4">{{ 'config.security_audit.title' | translate }}</h2>

      <div class="flex flex-wrap gap-3 mb-4 items-end">
        <div class="flex flex-col">
          <label class="text-sm text-muted-color mb-1">{{ 'layout.topbar.portrait_event' | translate }}</label>
          <p-select
            [options]="eventOptions"
            [(ngModel)]="filterEvent"
            [showClear]="true"
            optionLabel="label"
            optionValue="value"
            [placeholder]="'common.label.all' | translate"
            styleClass="w-56"
          ></p-select>
        </div>
        <div class="flex flex-col">
          <label class="text-sm text-muted-color mb-1">{{ 'config.security_audit.user_id_label' | translate }}</label>
          <input pInputText type="number" [(ngModel)]="filterUserId" class="w-32" />
        </div>
        <div class="flex flex-col">
          <label class="text-sm text-muted-color mb-1">{{ 'config.security_audit.date_range' | translate }}</label>
          <p-datepicker
            [(ngModel)]="filterRange"
            selectionMode="range"
            [showClear]="true"
            dateFormat="dd/mm/yy"
            styleClass="w-64"
          ></p-datepicker>
        </div>
        <button type="button" pButton icon="pi pi-search" (click)="applyFilter()"><span>{{ 'common.action.filter' | translate }}</span></button>
      </div>

      <p-table
        [value]="rows()"
        [lazy]="true"
        (onLazyLoad)="onLazyLoad($event)"
        [paginator]="true"
        [first]="first()"
        [rows]="perPage"
        [totalRecords]="total()"
        [loading]="loading()"
        [rowsPerPageOptions]="[20, 50, 100]"
        dataKey="id"
        styleClass="p-datatable-sm"
      >
        <ng-template #header>
          <tr>
            <th>{{ 'common.label.time' | translate }}</th>
            <th>{{ 'layout.topbar.portrait_event' | translate }}</th>
            <th>{{ 'config.security_audit.user_col' | translate }}</th>
            <th>{{ 'history.export.col_ip' | translate }}</th>
            <th>{{ 'common.label.detail' | translate }}</th>
          </tr>
        </ng-template>
        <ng-template #body let-row>
          <tr>
            <td class="whitespace-nowrap">{{ formatTime(row.created_at) }}</td>
            <td><p-tag [severity]="eventSeverity(row.event)" [value]="row.event"></p-tag></td>
            <td>
              @if (row.user_id) {
                <app-user-name [userId]="row.user_id" />
              } @else {
                {{ row.username || '—' }}
              }
            </td>
            <td class="whitespace-nowrap">{{ row.ip || '—' }}</td>
            <td class="text-xs text-muted-color">{{ formatDetail(row.detail) }}</td>
          </tr>
        </ng-template>
        <ng-template #emptymessage>
          <tr>
            <td colspan="5" class="text-center py-6 text-muted-color">
              {{ 'common.message.no_data' | translate }}
            </td>
          </tr>
        </ng-template>
      </p-table>
    </div>
  `,
})
/** ⚠ KHÔNG gọi `load()` lúc khởi tạo — `p-table` `onLazyLoad` tự bắn lần đầu. */
export class SecurityAudit extends BaseApp {
  rows = signal<AuditRow[]>([]);
  total = signal(0);
  first = signal(0);
  perPage = 20;

  filterEvent: string | null = null;
  filterUserId: number | null = null;
  filterRange: Date[] | null = null;

  private page = 1;

  readonly eventOptions = EVENTS.map((e) => ({ label: e, value: e }));

  onLazyLoad(e: TableLazyLoadEvent): void {
    const first = e.first ?? 0;
    this.perPage = e.rows ?? 20;
    this.page = Math.floor(first / this.perPage) + 1;
    this.load();
  }

  applyFilter(): void {
    this.page = 1;
    this.first.set(0);
    this.load();
  }

  private load(): void {
    this.loading.set(true);
    const params: Record<string, any> = {
      page: this.page,
      'per-page': this.perPage,
      event: this.filterEvent,
      user_id: this.filterUserId,
    };
    if (this.filterRange?.[0]) params['from'] = Math.floor(this.filterRange[0].getTime() / 1000);
    if (this.filterRange?.[1]) {
      // Cuối ngày để bao trọn ngày kết thúc.
      const end = new Date(this.filterRange[1]);
      end.setHours(23, 59, 59, 999);
      params['to'] = Math.floor(end.getTime() / 1000);
    }

    this.authService
      .getAuditLog(params)
      .pipe(finalize(() => this.loading.set(false)))
      .subscribe({
        next: (res: any) => {
          this.rows.set(res?.items ?? []);
          this.total.set(res?._meta?.totalCount ?? 0);
        },
        error: () => {
          this.rows.set([]);
          this.total.set(0);
        },
      });
  }

  formatTime(ts: number): string {
    return ts ? new Date(ts * 1000).toLocaleString() : '—';
  }

  formatDetail(detail: any): string {
    if (!detail) return '';
    try {
      const obj = typeof detail === 'string' ? JSON.parse(detail) : detail;
      return Object.entries(obj)
        .map(([k, v]) => `${k}=${v}`)
        .join(', ');
    } catch {
      return String(detail);
    }
  }

  eventSeverity(event: string): 'success' | 'warn' | 'danger' | 'info' {
    if (event === 'login_success' || event === 'new_device') return 'success';
    if (event === 'login_failed' || event === 'login_no_fingerprint') return 'warn';
    if (
      event === 'access_ip_mismatch' ||
      event === 'revoked_token_reused' ||
      event === 'ip_not_allowed' ||
      event === 'refresh_fp_mismatch'
    )
      return 'danger';
    return 'info';
  }
}
