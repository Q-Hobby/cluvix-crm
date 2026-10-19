// 📁 share/services/api-error-handler.service.ts
import { inject, Service, signal } from '@angular/core';
import { Router } from '@angular/router';

import { NotificationService } from '@/share/services/notification.service';

@Service()
export class ApiErrorHandlerService {
  lastError = signal<any | null>(null);
  private router = inject(Router);
  private noti = inject(NotificationService);

  handle(error: any, config: { silent?: boolean; redirect404?: string } = {}): never {
    this.lastError.set(error);

    const status = error?.status ?? 0;
    const message = error?.error?.message || 'Đã xảy ra lỗi.';

    // 🔔 Hiển thị Toast nếu không silent
    if (!config.silent) {
      this.noti.error(`Lỗi ${status}`, message);
    }

    // 🔁 Redirect nếu là 404 và cấu hình sẵn
    if (status === 404 && config.redirect404) {
      void this.router.navigateByUrl(config.redirect404);
    }

    // ❌ Ném lỗi lại để tiếp tục chain pipe
    throw error;
  }
}
