import { HttpClient, HttpContext, HttpErrorResponse } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { TranslateService } from '@ngx-translate/core';

import {
  SKIP_ERROR_NOTIFICATION,
  SKIP_GLOBAL_403,
  SKIP_GLOBAL_404,
} from '@/core/interceptors/response-handler.interceptor';
import { NotificationService } from '@/share/services/notification.service';

/**
 * Tải file export Excel/CSV mà BE trả về dưới dạng URL (`?_export=1` → `"{DOWNLOAD_URL}/api/download/export/<tên>"`).
 *
 * Endpoint tải CÓ JWT + chỉ phục vụ file của chính người tạo (BE S-2/O-6, 2026-10-18) — điều hướng
 * trình duyệt thẳng tới URL (`window.location.href = url`) KHÔNG mang header Authorization nên nhận 401.
 * Tải qua HttpClient (authInterceptor gắn Bearer), nhận Blob rồi lưu bằng thẻ `<a download>`.
 * KHÔNG đính token vào URL (frontend-hard-rules B.6).
 */
@Injectable({ providedIn: 'root' })
export class ExportFileService {
  private readonly http = inject(HttpClient);
  private readonly noti = inject(NotificationService);
  private readonly translate = inject(TranslateService);

  download(url: string | null | undefined): void {
    if (!url || typeof url !== 'string') return;
    // Lỗi tải là CỤC BỘ (file đã bị dọn/hết hạn, không phải của mình): báo tại chỗ, không đá cả trang
    // sang /auth/notfound hay /auth/accessdenied như request nuôi trang.
    const context = new HttpContext()
      .set(SKIP_GLOBAL_404, true)
      .set(SKIP_GLOBAL_403, true)
      .set(SKIP_ERROR_NOTIFICATION, true);
    this.http.get(url, { responseType: 'blob', context }).subscribe({
      next: (blob) => saveBlob(blob, exportFileNameFromUrl(url)),
      error: (err: HttpErrorResponse) => {
        // 401 đã do authInterceptor xử lý (refresh/đăng xuất) — không báo trùng.
        if (err?.status === 401) return;
        this.noti.errorMsg(this.translate.instant('common.message.export_file_unavailable'));
      },
    });
  }
}

/** Tên file = phần cuối của URL (BE sinh tên an toàn `[A-Za-z0-9_-]+.(xlsx|csv)`), bỏ query/hash. */
export function exportFileNameFromUrl(url: string): string {
  const path = url.split(/[?#]/)[0];
  const name = decodeURIComponent(path.substring(path.lastIndexOf('/') + 1));
  return name || 'export.xlsx';
}

export function saveBlob(blob: Blob, fileName: string): void {
  const objectUrl = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = objectUrl;
  a.download = fileName;
  document.body.appendChild(a);
  a.click();
  a.remove();
  // Trì hoãn thu hồi: một số trình duyệt bắt đầu tải bất đồng bộ sau click.
  setTimeout(() => URL.revokeObjectURL(objectUrl), 1000);
}
