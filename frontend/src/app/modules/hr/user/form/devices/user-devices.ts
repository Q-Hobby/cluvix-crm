import { ChangeDetectionStrategy, Component, OnInit, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TranslatePipe } from '@ngx-translate/core';
import { ButtonModule } from 'primeng/button';
import { TagModule } from 'primeng/tag';
import { finalize } from 'rxjs';

import { BaseApp } from '@/share/base/BaseApp';

interface DeviceRow {
  id: number;
  device_name: string | null;
  last_ip: string | null;
  last_seen: number | null;
  created_at: number | null;
  is_active: boolean;
}

/**
 * Danh sách thiết bị đã đăng nhập của user + đăng xuất từng thiết bị (device binding).
 * Mở qua dialogWrapperService từ menu profile.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  standalone: true,
  imports: [CommonModule, ButtonModule, TagModule, TranslatePipe],
  template: `
    <div class="flex flex-col gap-2 min-w-[320px]">
      @if (loading()) {
        <div class="text-center py-6 text-muted-color">
          <i class="pi pi-spin pi-spinner mr-2"></i>{{ 'common.label.loading' | translate }}
        </div>
      } @else if (devices().length === 0) {
        <div class="text-center py-6 text-muted-color">{{ 'common.message.no_data' | translate }}</div>
      } @else {
        @for (d of devices(); track d.id) {
          <div class="flex items-center justify-between gap-3 border border-surface-200 rounded-lg px-3 py-2">
            <div class="flex flex-col">
              <div class="flex items-center gap-2">
                <i class="pi pi-desktop text-muted-color"></i>
                <strong>{{ d.device_name || ('hr.user_devices.unknown_device' | translate) }}</strong>
                @if (d.is_active) {
                  <p-tag severity="success" [value]="'hr.user_devices.active' | translate"></p-tag>
                }
              </div>
              <small class="text-muted-color">
                {{ d.last_ip || '—' }} · {{ formatTime(d.last_seen) }}
              </small>
            </div>
            <button
              pButton
              type="button"
              severity="danger"
              [text]="true"
              [disabled]="revokingId() === d.id"
              (click)="revoke(d)"
            ><span>{{ 'common.action.logout' | translate }}</span></button>
          </div>
        }
      }
    </div>
  `,
})
export class UserDevices extends BaseApp implements OnInit {
  // `loading` kế thừa từ BaseApp (signal(true))
  devices = signal<DeviceRow[]>([]);
  revokingId = signal<number | null>(null);

  ngOnInit(): void {
    this.load();
  }

  private load(): void {
    this.loading.set(true);
    this.authService
      .listDevices()
      .pipe(finalize(() => this.loading.set(false)))
      .subscribe({
        next: (rows) => this.devices.set(rows ?? []),
        error: () => this.devices.set([]),
      });
  }

  formatTime(ts: number | null): string {
    if (!ts) return '—';
    return new Date(ts * 1000).toLocaleString();
  }

  revoke(d: DeviceRow): void {
    this.confirmationService.confirm({
      message: this.translate.instant('hr.user_devices.confirm_revoke'),
      accept: () => {
        this.revokingId.set(d.id);
        this.authService
          .revokeDevice(d.id)
          .pipe(finalize(() => this.revokingId.set(null)))
          .subscribe({
            next: () => {
              this.noti.successMsg(this.translate.instant('hr.user_devices.revoked'));
              this.devices.update((list) => list.filter((x) => x.id !== d.id));
            },
          });
      },
    });
  }
}
