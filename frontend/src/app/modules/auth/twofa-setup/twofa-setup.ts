import { ChangeDetectionStrategy, Component, OnInit, inject, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { TranslatePipe, TranslateService } from '@ngx-translate/core';
import { ButtonModule } from 'primeng/button';
import { InputTextModule } from 'primeng/inputtext';
import { QrCodeComponent } from 'ng-qrcode';
import { finalize } from 'rxjs';

import { AuthService } from '@/core/services/auth/auth.service';
import { AUTH_GRADIENT_BG } from '@/core/services/auth/auth-theme';
import { NotificationService } from '@/share/services/notification.service';

/**
 * Trang thiết lập / quản lý 2FA (TOTP). Dùng cho: bật tự nguyện (profile), ép-setup
 * (redirect khi API trả 2fa_setup_required), và tắt 2FA.
 * Check status trước — user đã bật KHÔNG gọi setup lại (tránh ghi đè secret đang dùng).
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  standalone: true,
  imports: [CommonModule, FormsModule, TranslatePipe, ButtonModule, InputTextModule, QrCodeComponent],
  template: `
    <div style="min-height:100vh;display:flex;align-items:center;justify-content:center;padding:20px;background:${AUTH_GRADIENT_BG}">
      <div class="bg-surface-0 dark:bg-surface-900" style="border-radius:20px;padding:32px;max-width:440px;width:100%;box-shadow:0 20px 60px rgba(0,0,0,.3)">
        <h2 class="text-[var(--p-text-color)]" style="font-size:1.6923rem;font-weight:700;margin-bottom:6px">{{ 'auth.twofa.setup_title' | translate }}</h2>

        @if (loading()) {
          <p class="text-[var(--p-text-muted-color)]" style="text-align:center;padding:24px"><i class="pi pi-spin pi-spinner mr-2"></i>{{ 'common.label.loading' | translate }}</p>
        } @else if (enabled()) {
          <!-- Đã bật → hiển thị + cho tắt -->
          <p style="color:#059669;font-weight:600;margin:12px 0"><i class="pi pi-check-circle mr-2"></i>{{ 'auth.twofa.enabled_state' | translate }}</p>
          <input pInputText [(ngModel)]="code" inputmode="numeric" autocomplete="one-time-code" class="w-full mb-3"
            [attr.aria-label]="'auth.twofa.code_placeholder' | translate"
            [placeholder]="'auth.twofa.code_placeholder' | translate" />
          <div style="display:flex;gap:8px">
            <button type="button" pButton severity="danger" [outlined]="true" class="flex-1" [disabled]="busy()"
              (click)="disable()">
              <i class="pi" [class.pi-spin]="busy()" [class.pi-spinner]="busy()" [class.pi-lock-open]="!busy()"></i>
              <span>{{ 'auth.twofa.disable' | translate }}</span>
            </button>
            <button type="button" pButton [text]="true" class="flex-1" (click)="finish()"><span>{{ 'auth.twofa.done' | translate }}</span></button>
          </div>
        } @else if (backupCodes(); as codes) {
          <!-- Bước 3: backup codes -->
          <p class="text-[var(--p-text-color)]" style="font-weight:600;margin:12px 0 8px">{{ 'auth.twofa.backup_title' | translate }}</p>
          <p style="color:#B45309;font-size:1rem;margin-bottom:12px">{{ 'auth.twofa.backup_warn' | translate }}</p>
          <div class="bg-surface-100 dark:bg-surface-800 text-[var(--p-text-color)]" style="display:grid;grid-template-columns:1fr 1fr;gap:8px;font-family:monospace;font-size:1.1538rem;padding:14px;border-radius:10px;margin-bottom:12px">
            @for (c of codes; track c) { <span>{{ c }}</span> }
          </div>
          <button type="button" pButton [outlined]="true" class="w-full mb-3" (click)="copyBackupCodes(codes)">
            <i class="pi pi-copy"></i><span>{{ 'common.action.copy' | translate }}</span>
          </button>
          <button type="button" pButton class="w-full" (click)="finish()"><span>{{ 'auth.twofa.done' | translate }}</span></button>
        } @else {
          <!-- Bước 1+2: QR + nhập mã -->
          <p class="text-[var(--p-text-muted-color)]" style="font-size:1.0769rem;margin-bottom:16px">{{ 'auth.twofa.setup_desc' | translate }}</p>
          @if (uri()) {
            <div style="display:flex;justify-content:center;margin-bottom:14px">
              <qr-code [value]="uri()!" [size]="180" errorCorrectionLevel="M" />
            </div>
            <p class="text-[var(--p-text-muted-color)]" style="font-size:1rem;text-align:center;margin-bottom:4px">{{ 'auth.twofa.manual_hint' | translate }}</p>
            <p class="bg-surface-100 dark:bg-surface-800 text-[var(--p-text-color)]" style="font-family:monospace;font-size:1.0769rem;text-align:center;word-break:break-all;padding:8px;border-radius:8px;margin-bottom:16px">{{ secret() }}</p>
          }
          <input pInputText [(ngModel)]="code" inputmode="numeric" autocomplete="one-time-code"
            class="w-full mb-3" [attr.aria-label]="'auth.twofa.code_placeholder' | translate"
            [placeholder]="'auth.twofa.code_placeholder' | translate" />
          <button type="button" pButton class="w-full" [disabled]="busy() || !uri()"
            (click)="verify()">
            <i class="pi" [class.pi-spin]="busy()" [class.pi-spinner]="busy()" [class.pi-check]="!busy()"></i>
            <span>{{ 'auth.twofa.verify' | translate }}</span>
          </button>
        }
      </div>
    </div>
  `,
})
export class TwofaSetup implements OnInit {
  private authService = inject(AuthService);
  private noti = inject(NotificationService);
  private translate = inject(TranslateService);

  loading = signal(true);
  enabled = signal(false);
  uri = signal<string | null>(null);
  secret = signal('');
  code = '';
  busy = signal(false);
  backupCodes = signal<string[] | null>(null);

  ngOnInit() {
    this.authService.get2FAStatus().subscribe({
      next: (s) => {
        if (s.enabled) {
          this.enabled.set(true);
          this.loading.set(false);
        } else {
          this.startEnroll();
        }
      },
      error: () => this.startEnroll(),
    });
  }

  private startEnroll() {
    this.authService.setup2FA().pipe(finalize(() => this.loading.set(false))).subscribe({
      next: (res) => {
        this.uri.set(res.uri);
        this.secret.set(res.secret);
      },
      error: () => this.noti.errorMsg(this.translate.instant('common.message.error')),
    });
  }

  copyBackupCodes(codes: string[]) {
    navigator.clipboard.writeText(codes.join('\n')).then(() => {
      this.noti.successMsg(this.translate.instant('common.message.copy_success'));
    });
  }

  verify() {
    const code = this.code.trim();
    if (!code || this.busy()) return;
    this.busy.set(true);
    this.authService.verifySetup2FA(code).pipe(finalize(() => this.busy.set(false))).subscribe({
      next: (res) => this.backupCodes.set(res.backup_codes ?? []),
      error: () => this.noti.errorMsg(this.translate.instant('auth.twofa.invalid_code')),
    });
  }

  disable() {
    const code = this.code.trim();
    if (!code || this.busy()) return;
    this.busy.set(true);
    this.authService.disable2FA(code).pipe(finalize(() => this.busy.set(false))).subscribe({
      next: () => {
        this.noti.successMsg(this.translate.instant('auth.twofa.disabled_ok'));
        this.finish();
      },
      error: () => this.noti.errorMsg(this.translate.instant('auth.twofa.invalid_code')),
    });
  }

  finish() {
    window.location.href = '/'; // reload để middleware bỏ chặn must-setup
  }
}
