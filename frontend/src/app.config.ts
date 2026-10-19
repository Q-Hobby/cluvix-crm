import {HttpClient, provideHttpClient, withFetch, withInterceptors} from '@angular/common/http';
import {
  APP_INITIALIZER,
  ApplicationConfig,
  ErrorHandler,
  importProvidersFrom,
  provideZoneChangeDetection,
} from '@angular/core';
import {provideAnimationsAsync} from '@angular/platform-browser/animations/async';

import {provideRouter, withEnabledBlockingInitialNavigation, withInMemoryScrolling, withPreloading, withViewTransitions} from '@angular/router';
import {SelectivePreloadStrategy} from '@/core/services/selective-preload.strategy';
import {primeuiLicense} from '@/share/utils/primeui-license.util';
import {environment} from '@env/environment';
import {provideTranslateService, TranslateLoader, TranslateService} from '@ngx-translate/core';
import {firstValueFrom} from 'rxjs';
import {CachedTranslateLoader} from '@/core/services/cached-translate-loader';
import {StaticTranslateLoader} from '@/core/services/static-translate-loader';
import {definePreset} from '@primeuix/themes';
// v22 mặc định base font-size 16px — dùng compat variant giữ 14px như trước (quyết định đã chốt, xem .skill/bmad/primeng-v22-migration/brief.md).
import Material from '@primeuix/themes/material-compat';
import {ConfirmationService, MessageService} from 'primeng/api';
import {providePrimeNG} from 'primeng/config';
import {DatePickerModule} from 'primeng/datepicker';
import {DialogService, DynamicDialogRef} from 'primeng/dynamicdialog';
import {IconFieldModule} from 'primeng/iconfield';
import {InputIconModule} from 'primeng/inputicon';
import {ToastModule} from 'primeng/toast';

import {appRoutes} from './app.routes';

import {GlobalErrorHandler} from '@/core/handler/global-error-handler';
import {apiDedupeInterceptor} from '@/core/interceptors/api-dedupe.interceptor';
import {authInterceptor} from '@/core/interceptors/auth.interceptor';
import {loadingInterceptor} from '@/core/interceptors/loading.interceptor';
import {requestLengthInterceptor} from '@/core/interceptors/request-length.interceptor';
import {responseHandlerInterceptor} from '@/core/interceptors/response-handler.interceptor';

import {AuthService} from '@/core/services/auth/auth.service';
import {TokenService} from '@/core/services/auth/token.service';
import {HeaderSyncService} from '@/core/services/header-sync.service';

function initializeAuth(tokenService: TokenService, authService: AuthService) {
  // Sequential: decrypt tokens TRƯỚC, restore auth user SAU.
  // Tránh race condition: nếu restoreAuthUser chạy trước initialize(), signals null
  // → Case 3 (no token + cached) sẽ clear data + redirect /login sai.
  return async () => {
    await tokenService.initialize();
    await authService.restoreAuthUser(); // await IDB read — authUser set trước preloadIndexedDb chạy
  };
}

function initializeTranslations(translate: TranslateService) {
  // Block app bootstrap đến khi default language translations load xong.
  // Tránh FOUC: components render với key strings trước khi translations sẵn sàng.
  // Default lang lấy từ localStorage 'lng' (StorageService lưu JSON.stringify), fallback 'vi-VN'.
  // PROD: vi-VN inline → instant. EN: lazy chunk (~50-100KB gzip).
  return async () => {
    const ALLOWED = ['vi-VN', 'en-US'];
    let lang = 'vi-VN';
    try {
      const raw = localStorage.getItem('lng');
      if (raw) {
        const parsed = JSON.parse(raw);
        if (typeof parsed === 'string' && ALLOWED.includes(parsed)) lang = parsed;
      }
    } catch {
      // localStorage không khả dụng hoặc value không phải JSON → dùng default
    }
    translate.setDefaultLang('vi-VN');
    await firstValueFrom(translate.use(lang));
  };
}

function preloadIndexedDb(tokenService: TokenService, headerSync: HeaderSyncService, authService: AuthService) {
  // Pre-populate IndexedDB nếu rỗng (version=0) trước khi component nào mount.
  // Chỉ chạy khi user có session hợp lệ — tránh request lỗi ở /login.
  // ensureSyncedIfEmpty đã có timeout 8s + silent fail nên không block app boot lâu.
  return async () => {
    const hasSession = tokenService.hasValidToken || tokenService.hasValidRefreshSession();
    if (!hasSession) return;
    await headerSync.ensureSyncedIfEmpty();
    // Load hồ sơ doanh nghiệp (story-43, thay authCompanyActive) — không phụ thuộc IndexedDB nên
    // gọi song song về nguyên tắc, nhưng giữ thứ tự sau sync để không đổi hành vi ngoài phạm vi.
    await authService.refreshBusinessProfile();
  };
}


const MyPreset = definePreset(Material, {
  semantic: {
    primary: {
      50: '{indigo.50}',
      100: '{indigo.100}',
      200: '{indigo.200}',
      300: '{indigo.300}',
      400: '{indigo.400}',
      500: '{indigo.500}',
      600: '{indigo.600}',
      700: '{indigo.700}',
      800: '{indigo.800}',
      900: '{indigo.900}',
      950: '{indigo.950}',
    },
  },
});

export const appConfig: ApplicationConfig = {
  providers: [
    provideZoneChangeDetection({
      eventCoalescing: true,
      runCoalescing: true,
    }),
    provideAnimationsAsync(),
    {provide: ErrorHandler, useClass: GlobalErrorHandler},
    {
      provide: APP_INITIALIZER,
      useFactory: initializeAuth,
      deps: [TokenService, AuthService],
      multi: true,
    },
    {
      provide: APP_INITIALIZER,
      useFactory: initializeTranslations,
      deps: [TranslateService],
      multi: true,
    },
    {
      provide: APP_INITIALIZER,
      useFactory: preloadIndexedDb,
      deps: [TokenService, HeaderSyncService, AuthService],
      multi: true,
    },


    // PrimeNG services
    DialogService,
    ConfirmationService,
    MessageService,
    DynamicDialogRef,

    // ngx-translate v17+
    // Hybrid loader (Sprint A3 + B):
    //  - PROD: StaticTranslateLoader → vi-VN inline trong main.js (0 HTTP), en-US lazy chunk
    //  - DEV:  CachedTranslateLoader → HTTP + localStorage SWR (translator-friendly hot reload)
    // Fallback vi-VN: missing key ở en-US fallback bản VN thay vì raw key string.
    provideTranslateService({
      loader: {
        provide: TranslateLoader,
        useFactory: (http: HttpClient) =>
          environment.prod ? new StaticTranslateLoader() : new CachedTranslateLoader(http),
        deps: [HttpClient],
      },
      fallbackLang: 'vi-VN',
    }),

    // Modules dùng chung
    importProvidersFrom(
      DatePickerModule,
      IconFieldModule,
      InputIconModule,
      ToastModule,
    ),

    // Router
    provideRouter(
      appRoutes,
      withInMemoryScrolling({anchorScrolling: 'enabled', scrollPositionRestoration: 'enabled'}),
      withEnabledBlockingInitialNavigation(),
      withPreloading(SelectivePreloadStrategy),
      // Cross-fade mượt giữa các route (native browser View Transitions API — độc lập @angular/animations).
      // skipInitialTransition: bỏ qua lần load app đầu tiên, tránh hiệu ứng thừa lúc cold boot.
      withViewTransitions({skipInitialTransition: true}),
    ),

    // HttpClient + interceptors
    provideHttpClient(
      withFetch(),
      withInterceptors([
        authInterceptor,
        apiDedupeInterceptor,
        responseHandlerInterceptor,
        requestLengthInterceptor,
        loadingInterceptor,
      ]),
    ),

    // PrimeNG theme
    providePrimeNG({
      ripple: false,
      theme: {preset: MyPreset, options: {darkModeSelector: '.app-dark'}},
      zIndex: {modal: 1100, overlay: 1000, menu: 1000, tooltip: 1100},
      // PrimeUI Community license key (v22+ bắt buộc). Đọc LAZY qua getter, KHÔNG gán trực tiếp
      // environment.primeuiLicense: providePrimeNG chỉ lấy `license` bên trong app initializer
      // (lúc bootstrap), nên tới lúc đó resolvePrimeuiLicense() ở main.ts đã chốt xong key runtime
      // (cache CDN hoặc fallback build-time). Gán trực tiếp sẽ đóng băng giá trị lúc load module.
      // Xem share/utils/primeui-license.util.ts.
      get license() {
        return primeuiLicense();
      },
    }),
  ],
};
