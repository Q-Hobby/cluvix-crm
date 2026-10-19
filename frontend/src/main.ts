import { registerLocaleData } from '@angular/common';
import localeEn from '@angular/common/locales/en';
import localeVi from '@angular/common/locales/vi';
import { enableProdMode, provideZoneChangeDetection } from '@angular/core';
import { bootstrapApplication } from '@angular/platform-browser';
import { environment } from '@env/environment';

import { AppComponent } from './app.component';
import { appConfig } from './app.config';

import { setupRouteMeta } from '@/share/utils/setup-route-meta';
import { refreshPrimeuiLicenseFromCdn, resolvePrimeuiLicense } from '@/share/utils/primeui-license.util';

registerLocaleData(localeVi, 'vi-VN');
registerLocaleData(localeEn, 'en-US');

if (environment.prod) {
  enableProdMode();
}

// ✅ Đăng ký Service Worker và reload ngay khi có worker mới được active
if ('serviceWorker' in navigator && environment.prod) {
  // Đảm bảo tất cả các tab đều lắng nghe và tự reload khi nhận message
  navigator.serviceWorker.addEventListener('message', (event) => {
    if (event.data?.type === 'SW_UPDATED') {
      window.location.reload();
    }
  });

  window.addEventListener('load', () => {
    navigator.serviceWorker.register('/custom-ngsw-worker.js').catch((err: any) => {
      if (!environment.prod) {
        console.warn('[SW] Registration failed ❌', err);
      }
    });
  });
}

// Chốt key PrimeUI TRƯỚC bootstrap: providePrimeNG đọc `license` ngay trong app initializer.
// Đồng bộ, không chạm mạng — xem share/utils/primeui-license.util.ts.
resolvePrimeuiLicense();

bootstrapApplication(AppComponent, {
  ...appConfig,
  providers: [
    ...(appConfig.providers || []),
    provideZoneChangeDetection({ eventCoalescing: true, runCoalescing: true }), // Gộp (coalesce) & giảm số lần change detection toàn app
  ],
})
  .then((appRef: any) => {
    void setupRouteMeta(appRef.injector);
    // Lấy key mới từ CDN dưới nền, có hiệu lực từ lần tải trang sau. Không await, không chặn UI.
    void refreshPrimeuiLicenseFromCdn();
  })
  .catch((err: any) => console.error(err));
