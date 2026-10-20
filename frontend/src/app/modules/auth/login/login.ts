import { ChangeDetectionStrategy, Component, DestroyRef, inject, signal } from '@angular/core';
import { HttpErrorResponse } from '@angular/common/http';
import { FormBuilder, FormsModule, ReactiveFormsModule, Validators } from '@angular/forms';
import { Title } from '@angular/platform-browser';
import { ActivatedRoute, Router } from '@angular/router';
import { TranslatePipe, TranslateService } from '@ngx-translate/core';
import { ButtonModule } from 'primeng/button';
import { IconFieldModule } from 'primeng/iconfield';
import { InputIconModule } from 'primeng/inputicon';
import { InputTextModule } from 'primeng/inputtext';
import { Select } from 'primeng/select';
import { firstValueFrom } from 'rxjs';

import { AuthService } from '@/core/services/auth/auth.service';
import { AUTH_GRADIENT_BG } from '@/core/services/auth/auth-theme';
import { getDeviceFingerprint } from '@/core/services/auth/fingerprint';
import { safeInternalRedirect } from '@/core/services/auth/safe-redirect';
import { FORCE_CHANGE_PASSWORD_ROUTE } from '@/core/guards/authGuard';
import { NotificationService } from '@/share/services/notification.service';
import { StorageService } from '@/share/services/storage.service';
import { CONFIG_LOCALE_LANGUAGE_OPTIONS, STORAGE_KEY_SELECT } from '@/define';

@Component({
  template: `
    <style>
      /* ---- Shell: căn giữa cụm login trên màn hình lớn, tránh giãn cách quá mức ---- */
      .login-shell {
        min-height: 100vh;
        display: flex;
        align-items: center;
        justify-content: center;
      }

      .login-page {
        width: 100%;
        min-height: 100vh;
        display: flex;
      }

      @media (min-width: 1440px) {
        .login-shell {
          padding: 3rem;
          background: linear-gradient(
            135deg,
            color-mix(in srgb, #7b3f9e 6%, var(--p-content-background, #f9fafb)) 0%,
            var(--p-content-background, #f9fafb) 65%
          );
        }

        .login-page {
          max-width: 1360px;
          min-height: 0;
          height: min(860px, 88vh);
          border-radius: 1.5rem;
          overflow: hidden;
          box-shadow: 0 30px 80px rgba(0, 0, 0, 0.18);
        }
      }

      /* ---- Left panel: brand story (ẩn trên mobile) ---- */
      .login-brand-panel {
        display: none;
        position: relative;
        flex-direction: column;
        justify-content: center;
        gap: 2.5rem;
        width: 48%;
        padding: 4rem 3.5rem;
        background: ${AUTH_GRADIENT_BG};
        color: white;
        overflow: hidden;
      }

      .login-brand-panel::before {
        content: '';
        position: absolute;
        inset: 0;
        background-image:
          radial-gradient(circle at 8% 0%, rgba(255, 255, 255, 0.14) 0, transparent 42%),
          radial-gradient(circle at 100% 100%, rgba(0, 0, 0, 0.16) 0, transparent 55%);
        pointer-events: none;
      }

      /* Hoạ tiết icon CRM mờ trang trí nền — cố định vị trí, không phải trang trí ngẫu nhiên/rình rang */
      .login-brand-pattern {
        position: absolute;
        inset: 0;
        color: rgba(255, 255, 255, 0.1);
        pointer-events: none;
      }

      .login-brand-pattern i {
        position: absolute;
      }

      .login-brand-pattern .login-pattern-a {
        top: 6%;
        right: 9%;
        font-size: 4.5rem;
      }

      .login-brand-pattern .login-pattern-b {
        bottom: 8%;
        left: 5%;
        font-size: 4rem;
      }

      .login-brand-pattern .login-pattern-c {
        top: 42%;
        right: 3%;
        font-size: 2.25rem;
        opacity: 0.6;
      }

      .login-brand-pattern .login-pattern-d {
        bottom: 22%;
        right: 15%;
        font-size: 2.5rem;
        opacity: 0.7;
      }

      .login-brand-pattern .login-pattern-e {
        top: 11%;
        left: 7%;
        font-size: 2rem;
        opacity: 0.6;
      }

      .login-brand-pattern .login-pattern-f {
        bottom: 5%;
        right: 36%;
        font-size: 1.75rem;
        opacity: 0.5;
      }

      .login-brand-content {
        position: relative;
        z-index: 1;
        display: flex;
        flex-direction: column;
        gap: 2.25rem;
      }

      .login-brand-logo {
        display: flex;
        align-items: center;
        gap: 1rem;
      }

      .login-brand-logo-icon {
        position: relative;
        display: inline-flex;
        align-items: center;
        justify-content: center;
        width: 3.25rem;
        height: 3.25rem;
        border-radius: 0.9rem;
        background: linear-gradient(135deg, rgba(255, 255, 255, 0.28), rgba(255, 255, 255, 0.08));
        border: 1px solid rgba(255, 255, 255, 0.35);
        box-shadow:
          0 0 0 6px rgba(255, 255, 255, 0.06),
          0 8px 24px rgba(0, 0, 0, 0.18);
      }

      .login-brand-logo-icon i {
        font-size: 1.7rem;
      }

      .login-brand-name {
        font-size: 1.3846rem;
        font-weight: 700;
        letter-spacing: 0.03em;
      }

      .login-brand-heading {
        font-size: 2.3077rem;
        font-weight: 800;
        line-height: 1.2;
        letter-spacing: -0.01em;
      }

      .login-brand-tagline {
        margin-top: 0.75rem;
        font-size: 1.1538rem;
        color: rgba(255, 255, 255, 0.85);
        line-height: 1.5;
        max-width: 28rem;
      }

      .login-feature-list {
        display: flex;
        flex-direction: column;
      }

      .login-feature-item {
        display: flex;
        align-items: flex-start;
        gap: 1rem;
        padding: 1.125rem 0;
      }

      .login-feature-item + .login-feature-item {
        border-top: 1px solid rgba(255, 255, 255, 0.14);
      }

      .login-feature-icon {
        flex-shrink: 0;
        display: inline-flex;
        align-items: center;
        justify-content: center;
        width: 2.5rem;
        height: 2.5rem;
        border-radius: 0.65rem;
        background: rgba(255, 255, 255, 0.12);
        color: white;
      }

      .login-feature-icon i {
        font-size: 1.1538rem;
      }

      .login-feature-title {
        font-weight: 700;
        font-size: 1.0769rem;
      }

      .login-feature-desc {
        color: rgba(255, 255, 255, 0.75);
        font-size: 0.9231rem;
        line-height: 1.45;
        margin-top: 0.15rem;
      }

      /* ---- Right panel: form ---- */
      .login-form-panel {
        position: relative;
        flex: 1;
        display: flex;
        align-items: center;
        justify-content: center;
        padding: 2rem 1.5rem;
      }

      .login-lang-switcher {
        position: absolute;
        top: 1.25rem;
        right: 1.25rem;
        z-index: 2;
      }

      .login-lang-item {
        display: flex;
        align-items: center;
        gap: 0.5rem;
      }

      .login-lang-item img.flag {
        border-radius: 0.15rem;
        box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.08);
      }

      .login-form-wrap {
        width: 100%;
        max-width: 36rem;
      }

      .login-form-card {
        padding: 2.5rem 2.25rem;
        border-radius: 1.25rem;
        box-shadow: 0 1px 2px rgba(0, 0, 0, 0.04);
      }

      @media (max-width: 480px) {
        .login-form-card {
          padding: 2rem 1.5rem;
        }
      }

      .login-mobile-brand {
        display: flex;
        align-items: center;
        gap: 0.75rem;
        margin-bottom: 2.5rem;
      }

      .login-mobile-brand-icon {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        width: 2.75rem;
        height: 2.75rem;
        border-radius: 0.75rem;
        background: ${AUTH_GRADIENT_BG};
        box-shadow: 0 6px 16px rgba(123, 63, 158, 0.3);
      }

      .login-mobile-brand-icon i {
        font-size: 1.4rem;
        color: white;
      }

      .login-mobile-brand-name {
        font-size: 1.2308rem;
        font-weight: 700;
        color: var(--p-text-color, #1f2937);
      }

      .login-title {
        font-size: 1.6923rem;
        font-weight: 700;
        color: var(--p-text-color, #1f2937);
        margin: 0 0 0.4rem;
      }

      .login-subtitle {
        color: var(--p-text-muted-color, #4b5563);
        font-size: 1rem;
        margin: 0 0 2rem;
        line-height: 1.4;
      }

      .login-form {
        display: flex;
        flex-direction: column;
        gap: 1.25rem;
      }

      .login-form :where(.p-iconfield) input.p-inputtext {
        width: 100%;
        padding-top: 0.875rem !important;
        padding-bottom: 0.875rem !important;
        border-radius: 0.7rem !important;
      }

      .login-password-field {
        position: relative;
      }

      .login-password-field :where(.p-iconfield) input.p-inputtext {
        padding-right: 2.75rem !important;
      }

      .login-password-toggle {
        position: absolute;
        top: 50%;
        right: 0.9rem;
        transform: translateY(-50%);
        z-index: 1;
        display: inline-flex;
        align-items: center;
        justify-content: center;
        background: none;
        border: none;
        padding: 0.25rem;
        color: var(--p-text-muted-color, #6b7280);
        cursor: pointer;
      }

      .login-password-toggle:hover {
        color: var(--p-text-color, #1f2937);
      }

      .login-remember {
        display: flex;
        align-items: center;
        gap: 0.5rem;
        font-size: 0.9231rem;
        color: var(--p-text-muted-color, #4b5563);
        cursor: pointer;
      }

      /* Banner đếm ngược khi bị rate-limit — semantic Amber (trạng thái hệ thống, không phải brand) */
      .login-rate-limit {
        display: flex;
        align-items: center;
        gap: 0.5rem;
        margin: -0.5rem 0 0.25rem;
        padding: 0.625rem 0.875rem;
        border-radius: 0.6rem;
        background: color-mix(in srgb, #f59e0b 12%, var(--p-content-background, #fff));
        color: #b45309;
        font-size: 0.8846rem;
        font-weight: 600;
      }

      .login-rate-limit i {
        font-size: 0.9231rem;
      }

      .login-button {
        width: 100%;
        padding: 0.9rem !important;
        background: #7b3f9e !important;
        border: none !important;
        border-radius: 0.7rem !important;
        font-size: 1.0769rem !important;
        font-weight: 600 !important;
        box-shadow: 0 8px 20px rgba(123, 63, 158, 0.3) !important;
        margin-top: 0.25rem;
      }

      .login-button:hover:not(:disabled) {
        background: #6a3589 !important;
        box-shadow: 0 10px 24px rgba(123, 63, 158, 0.4) !important;
      }

      .login-security-badge {
        display: flex;
        align-items: center;
        justify-content: center;
        gap: 0.5rem;
        margin-top: 2rem;
        padding-top: 1.5rem;
        border-top: 1px solid var(--p-content-border-color, #e5e7eb);
        color: var(--p-text-muted-color, #6b7280);
        font-size: 0.8462rem;
      }

      .login-security-badge i {
        color: #10b981;
        font-size: 1rem;
      }

      @media (min-width: 992px) {
        .login-brand-panel {
          display: flex;
        }

        .login-mobile-brand {
          display: none;
        }
      }
    </style>

    <div class="login-shell">
    <div class="login-page">
      <div class="login-brand-panel">
        <div class="login-brand-pattern" aria-hidden="true">
          <i class="pi pi-chart-line login-pattern-a"></i>
          <i class="pi pi-users login-pattern-b"></i>
          <i class="pi pi-calendar login-pattern-c"></i>
          <i class="pi pi-shield login-pattern-d"></i>
          <i class="pi pi-comments login-pattern-e"></i>
          <i class="pi pi-star login-pattern-f"></i>
        </div>

        <div class="login-brand-content">
          <div class="login-brand-logo">
            <span class="login-brand-logo-icon"><i class="pi pi-chart-line"></i></span>
            <span class="login-brand-name">CLUVIX CRM</span>
          </div>

          <div>
            <h1 class="login-brand-heading">{{ 'auth.login.heading' | translate }}</h1>
            <p class="login-brand-tagline">{{ 'auth.login.tagline' | translate }}</p>
          </div>

          <div class="login-feature-list">
            <div class="login-feature-item">
              <span class="login-feature-icon"><i class="pi pi-shield" aria-hidden="true"></i></span>
              <div>
                <div class="login-feature-title">{{ 'auth.login.feature_security_title' | translate }}</div>
                <div class="login-feature-desc">{{ 'auth.login.feature_security_desc' | translate }}</div>
              </div>
            </div>
            <div class="login-feature-item">
              <span class="login-feature-icon"><i class="pi pi-sitemap" aria-hidden="true"></i></span>
              <div>
                <div class="login-feature-title">{{ 'auth.login.feature_multibranch_title' | translate }}</div>
                <div class="login-feature-desc">{{ 'auth.login.feature_multibranch_desc' | translate }}</div>
              </div>
            </div>
            <div class="login-feature-item">
              <span class="login-feature-icon"><i class="pi pi-users" aria-hidden="true"></i></span>
              <div>
                <div class="login-feature-title">{{ 'auth.login.feature_care_title' | translate }}</div>
                <div class="login-feature-desc">{{ 'auth.login.feature_care_desc' | translate }}</div>
              </div>
            </div>
          </div>

        </div>
      </div>

      <div class="login-form-panel bg-surface-50 dark:bg-surface-950">
        <div class="login-lang-switcher">
          <p-select
            [options]="CONFIG_LOCALE_LANGUAGE_OPTIONS"
            [ngModel]="selectedLang()"
            (onChange)="onLangChange($event.value)"
            optionLabel="label"
            optionValue="value"
            appendTo="body"
            size="small"
            [style]="{ width: '12rem' }"
            [attr.aria-label]="'common.label.switch_language' | translate"
          >
            <ng-template #selectedItem let-item>
              <div class="login-lang-item">
                <img
                  [class]="flagClass(item.value)"
                  src="/assets/overrides/images/flag-placeholder.svg"
                  [alt]="item.value"
                />
                <span>{{ item.label }}</span>
              </div>
            </ng-template>
            <ng-template #item let-item>
              <div class="login-lang-item">
                <img
                  [class]="flagClass(item.value)"
                  src="/assets/overrides/images/flag-placeholder.svg"
                  [alt]="item.value"
                />
                <span>{{ item.label }}</span>
              </div>
            </ng-template>
          </p-select>
        </div>

        <div class="login-form-wrap">
          <div class="login-mobile-brand">
            <span class="login-mobile-brand-icon"><i class="pi pi-chart-line"></i></span>
            <span class="login-mobile-brand-name">CLUVIX CRM</span>
          </div>

          <div class="login-form-card bg-surface-0 dark:bg-surface-900 border border-surface-200 dark:border-surface-700">
          @if (requires2fa()) {
            <h2 class="login-title">{{ 'auth.twofa.enter_code' | translate }}</h2>
            <p class="login-subtitle">{{ 'auth.twofa.setup_desc' | translate }}</p>
            @if (retryAfterSeconds() > 0) {
              <div class="login-rate-limit">
                <i class="pi pi-clock" aria-hidden="true"></i>
                <span>{{ 'auth.twofa.rate_limited' | translate: { seconds: retryAfterSeconds() } }}</span>
              </div>
            }
            <form class="login-form" (ngSubmit)="onVerify2fa()">
              <p-iconfield>
                <p-inputicon class="pi pi-shield" />
                <input
                  [(ngModel)]="code2fa"
                  name="code2fa"
                  autofocus
                  inputmode="numeric"
                  autocomplete="one-time-code"
                  pInputText
                  fluid
                  [attr.aria-label]="'auth.twofa.code_placeholder' | translate"
                  placeholder="{{ 'auth.twofa.code_placeholder' | translate }}"
                  type="text"
                />
              </p-iconfield>

              <label class="login-remember">
                <input type="checkbox" [(ngModel)]="rememberDevice" name="rememberDevice" />
                {{ 'auth.twofa.remember_device' | translate }}
              </label>

              <button
                [disabled]="isSubmitting() || retryAfterSeconds() > 0"
                class="login-button"
                pButton
                type="submit"
              >
                <i
                  class="pi"
                  [class.pi-spin]="isSubmitting()"
                  [class.pi-spinner]="isSubmitting()"
                  [class.pi-check]="!isSubmitting()"
                ></i>
                <span>
                  @if (isSubmitting()) {
                    {{ 'auth.login.verifying' | translate }}
                  } @else {
                    {{ 'auth.twofa.verify' | translate }}
                  }
                </span>
              </button>
            </form>
          } @else {
            <h2 class="login-title">{{ 'auth.login.title' | translate }}</h2>
            <p class="login-subtitle">{{ 'auth.login.subtitle' | translate }}</p>

            @if (retryAfterSeconds() > 0) {
              <div class="login-rate-limit">
                <i class="pi pi-clock" aria-hidden="true"></i>
                <span>{{ 'auth.login.rate_limited' | translate: { seconds: retryAfterSeconds() } }}</span>
              </div>
            }

            <form [formGroup]="form" class="login-form" (ngSubmit)="onLogin()">
              <p-iconfield>
                <p-inputicon class="pi pi-user" />
                <input
                  [class.ng-invalid]="form.controls['username'].invalid && form.controls['username'].touched"
                  autofocus
                  formControlName="username"
                  autocomplete="username"
                  pInputText
                  fluid
                  [attr.aria-label]="'common.label.username' | translate"
                  placeholder="{{ 'common.label.username' | translate }}"
                  type="text"
                />
              </p-iconfield>

              <div class="login-password-field">
                <p-iconfield>
                  <p-inputicon class="pi pi-lock" />
                  <input
                    [class.ng-invalid]="form.controls['password'].invalid && form.controls['password'].touched"
                    formControlName="password"
                    autocomplete="current-password"
                    pInputText
                    fluid
                    [attr.aria-label]="'common.label.password' | translate"
                    placeholder="{{ 'common.label.password' | translate }}"
                    [type]="showPassword() ? 'text' : 'password'"
                  />
                </p-iconfield>
                <button
                  type="button"
                  class="login-password-toggle"
                  (click)="togglePasswordVisibility()"
                  [attr.aria-label]="(showPassword() ? 'common.action.hide_password' : 'common.action.show_password') | translate"
                >
                  <i class="pi" [class.pi-eye]="!showPassword()" [class.pi-eye-slash]="showPassword()"></i>
                </button>
              </div>

              <button
                [disabled]="isSubmitting() || retryAfterSeconds() > 0"
                class="login-button"
                pButton
                type="submit"
              >
                <i
                  class="pi"
                  [class.pi-spin]="isSubmitting()"
                  [class.pi-spinner]="isSubmitting()"
                  [class.pi-sign-in]="!isSubmitting()"
                ></i>
                <span>
                  @if (isSubmitting()) {
                    {{ 'auth.login.submitting' | translate }}
                  } @else {
                    {{ 'auth.login.title' | translate }}
                  }
                </span>
              </button>
            </form>
          }

          <div class="login-security-badge">
            <i class="pi pi-verified" aria-hidden="true"></i>
            <span>{{ 'auth.login.security_badge' | translate }}</span>
          </div>
          </div>
        </div>
      </div>
    </div>
    </div>
  `,
  standalone: true,
  imports: [
    ButtonModule,
    IconFieldModule,
    InputIconModule,
    InputTextModule,
    Select,
    ReactiveFormsModule,
    FormsModule,
    TranslatePipe,
  ],
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class Login {
  isSubmitting = signal(false);
  private title = inject(Title);
  private translate = inject(TranslateService);
  private route = inject(ActivatedRoute);
  // authGuard set query param `redirect` (xem authGuard.ts:toLogin). Chỉ chấp nhận
  // path nội bộ bắt đầu bằng "/" và không phải "//..." để chống open-redirect.
  readonly redirectUrl = signal(safeInternalRedirect(this.route.snapshot.queryParamMap.get('redirect')));
  private router = inject(Router);
  private fb = inject(FormBuilder);
  form = this.fb.nonNullable.group({
    username: ['', Validators.required],
    password: ['', Validators.required],
  });
  private noti = inject(NotificationService);
  private authService = inject(AuthService);
  private destroyRef = inject(DestroyRef);
  private storageService = inject(StorageService);

  constructor() {
    this.title.setTitle('Cluvix CRM');
  }

  // 2FA challenge state
  requires2fa = signal(false);
  private challengeToken = '';
  code2fa = '';
  rememberDevice = false;

  showPassword = signal(false);

  togglePasswordVisibility() {
    this.showPassword.update((v) => !v);
  }

  // Rate-limit countdown (login thường + 2FA verify) — BE trả `retry_after` (giây) khi 429.
  retryAfterSeconds = signal(0);
  private countdownTimer?: ReturnType<typeof setInterval>;

  // Chọn ngôn ngữ ngay ở màn login (trước khi có token) — đổi UI + localStorage cục bộ ngay,
  // đồng thời gửi kèm lên BE lúc login/2FA để BE tự đồng bộ vào user_profile (xem onLogin).
  protected readonly CONFIG_LOCALE_LANGUAGE_OPTIONS = CONFIG_LOCALE_LANGUAGE_OPTIONS;
  selectedLang = signal(this.translate.currentLang || 'vi-VN');

  onLangChange(lang: string) {
    if (!lang || lang === this.selectedLang()) return;
    this.selectedLang.set(lang);
    this.translate.use(lang);
    this.storageService.saveStorage(STORAGE_KEY_SELECT.Lang, lang);
  }

  /** Class CSS cờ quốc gia (`overrides/_flags.scss`, dùng chung với module khác — vd sale/opportunity/portrait). */
  flagClass(locale: string): string {
    const country = locale?.split('-')[1]?.toLowerCase() ?? '';
    return `flag flag-${country}`;
  }

  async onLogin(event?: Event) {
    event?.preventDefault();

    if (this.isSubmitting() || this.retryAfterSeconds() > 0) return;

    this.form.markAllAsTouched();
    if (this.form.invalid) return;

    this.isSubmitting.set(true);
    const { username, password } = this.form.getRawValue();

    try {
      const device_fingerprint = await getDeviceFingerprint();
      // Gửi kèm locale đang chọn ở màn login — BE tự đồng bộ vào user_profile TRƯỚC khi trả
      // response (xem handler.go syncUserLocale), tránh phải PATCH lại sau khi có token.
      const body: any = await firstValueFrom(
        this.authService.login({ username, password, device_fingerprint, locale: this.selectedLang() }),
      );

      // Cần 2FA → chuyển sang màn nhập mã, chưa navigate.
      if (body?.requires_2fa) {
        this.challengeToken = body.challenge_token;
        this.requires2fa.set(true);
        this.isSubmitting.set(false);
        return;
      }

      await this.afterLoginSuccess(body);
    } catch (err) {
      this.form.patchValue({ password: '' });
      if (this.handleRateLimitError(err, 'auth.login.rate_limited')) {
        // đã hiện countdown, không toast generic nữa
      } else {
        this.noti.errorMsg(this.translate.instant('common.message.auth_error'));
      }
    } finally {
      this.isSubmitting.set(false);
    }
  }

  async onVerify2fa(event?: Event) {
    event?.preventDefault();
    if (this.isSubmitting() || this.retryAfterSeconds() > 0) return;
    const code = this.code2fa.trim();
    if (!code) return;

    this.isSubmitting.set(true);
    try {
      const body: any = await firstValueFrom(
        this.authService.login2fa({
          challenge_token: this.challengeToken,
          code,
          remember_device: this.rememberDevice,
        }),
      );
      await this.afterLoginSuccess(body);
    } catch (err) {
      this.code2fa = '';
      if (this.handleRateLimitError(err, 'auth.twofa.rate_limited')) {
        // đã hiện countdown, không toast generic nữa
      } else {
        this.noti.errorMsg(this.translate.instant('common.message.auth_error'));
      }
    } finally {
      this.isSubmitting.set(false);
    }
  }

  /** Trả true nếu là lỗi 429 rate-limit (đã start countdown) — caller khỏi toast thêm. */
  private handleRateLimitError(err: unknown, messageKey: string): boolean {
    if (!(err instanceof HttpErrorResponse) || err.status !== 429) return false;
    const retryAfter = Number(err.error?.retry_after) || 0;
    if (retryAfter <= 0) return false;
    this.startRetryCountdown(retryAfter);
    this.noti.warnMsg(this.translate.instant(messageKey, { seconds: retryAfter }));
    return true;
  }

  /** Đếm ngược mỗi giây — disable submit tới khi hết hạn, tự dọn interval khi component destroy. */
  private startRetryCountdown(seconds: number) {
    clearInterval(this.countdownTimer);
    this.retryAfterSeconds.set(seconds);
    this.countdownTimer = setInterval(() => {
      const next = this.retryAfterSeconds() - 1;
      if (next <= 0) {
        this.retryAfterSeconds.set(0);
        clearInterval(this.countdownTimer);
      } else {
        this.retryAfterSeconds.set(next);
      }
    }, 1000);
    this.destroyRef.onDestroy(() => clearInterval(this.countdownTimer));
  }

  /**
   * Sau khi có token: chuyển sang trang /syncing để đồng bộ IndexedDB rồi điều hướng tiếp.
   * Dùng chung login + 2FA. Locale đã được BE đồng bộ vào user_profile TRƯỚC khi trả response
   * (xem handler.go syncUserLocale) — body.user.userProfile.locale ở đây đã là giá trị mới nhất,
   * FE không cần tự PATCH lại.
   */
  private async afterLoginSuccess(body: any) {
    this.isSubmitting.set(false);
    this.noti.successMsg(this.translate.instant('auth.login.success'));
    // S-4/D-1 — bắt buộc đổi mật khẩu: vào thẳng màn đổi mật khẩu, KHÔNG qua /syncing (BE chặn mọi API
    // nghiệp vụ tới khi đổi xong; đổi xong form tự đăng xuất ⇒ đăng nhập lại mới đồng bộ).
    if (body?.user?.mustChangePassword) {
      await this.router.navigate([FORCE_CHANGE_PASSWORD_ROUTE], { replaceUrl: true });
      return;
    }
    const redirect = this.redirectUrl() || '/';
    await this.router.navigate(['/syncing'], { queryParams: { redirect }, replaceUrl: true });
  }
}
