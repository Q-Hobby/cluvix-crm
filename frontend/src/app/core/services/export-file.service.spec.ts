import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TranslateService } from '@ngx-translate/core';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ExportFileService, exportFileNameFromUrl } from './export-file.service';
import { SKIP_ERROR_NOTIFICATION, SKIP_GLOBAL_403, SKIP_GLOBAL_404 } from '@/core/interceptors/response-handler.interceptor';
import { NotificationService } from '@/share/services/notification.service';

/**
 * BE S-2/O-6 (2026-10-18): file export tải qua `GET /api/download/export/:name` CÓ JWT. FE phải tải
 * bằng HttpClient (authInterceptor gắn Bearer) — KHÔNG điều hướng trình duyệt tới URL thô (mất header
 * ⇒ 401) và KHÔNG đính token vào URL.
 */
describe('ExportFileService', () => {
  const url = 'https://crm.test/api/download/export/Khach-Hang-export-2026-10-18-10-00-00-0123456789abcdef0123456789abcdef.xlsx';
  let http: HttpTestingController;
  let errorMsg: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    errorMsg = vi.fn();
    TestBed.configureTestingModule({
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        { provide: NotificationService, useValue: { errorMsg } },
        { provide: TranslateService, useValue: { instant: (k: string) => k } },
      ],
    });
    http = TestBed.inject(HttpTestingController);
    (URL as any).createObjectURL = vi.fn(() => 'blob:x');
    (URL as any).revokeObjectURL = vi.fn();
  });

  afterEach(() => {
    http.verify();
    vi.restoreAllMocks();
  });

  it('GET blob đúng URL (không thêm token vào URL) rồi lưu với tên file của URL', () => {
    const clicked: string[] = [];
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) {
      clicked.push(this.download);
    });

    TestBed.inject(ExportFileService).download(url);

    const req = http.expectOne(url);
    expect(req.request.method).toBe('GET');
    expect(req.request.responseType).toBe('blob');
    expect(req.request.urlWithParams).toBe(url);
    expect(req.request.context.get(SKIP_GLOBAL_404)).toBe(true);
    req.flush(new Blob(['PK']));

    expect(clicked).toEqual([exportFileNameFromUrl(url)]);
    expect(errorMsg).not.toHaveBeenCalled();
  });

  it('file đã bị dọn (404) ⇒ báo lỗi tại chỗ', () => {
    TestBed.inject(ExportFileService).download(url);
    http.expectOne(url).flush(new Blob(['{}']), { status: 404, statusText: 'Not Found' });
    expect(errorMsg).toHaveBeenCalledWith('common.message.export_file_unavailable');
  });

  it('401 ⇒ KHÔNG báo lỗi trùng (authInterceptor đã xử lý) và request mang cờ bỏ 403/lỗi chung', () => {
    TestBed.inject(ExportFileService).download(url);
    const req = http.expectOne(url);
    expect(req.request.context.get(SKIP_GLOBAL_403)).toBe(true);
    expect(req.request.context.get(SKIP_ERROR_NOTIFICATION)).toBe(true);
    req.flush(new Blob(['{}']), { status: 401, statusText: 'Unauthorized' });
    expect(errorMsg).not.toHaveBeenCalled();
  });

  it('URL rỗng ⇒ không gửi request', () => {
    TestBed.inject(ExportFileService).download('');
    TestBed.inject(ExportFileService).download(undefined);
    http.expectNone(() => true);
  });

  it('exportFileNameFromUrl lấy basename, bỏ query/hash', () => {
    expect(exportFileNameFromUrl('https://h/api/download/export/a-1.csv?x=1#y')).toBe('a-1.csv');
    expect(exportFileNameFromUrl('https://h/api/download/export/')).toBe('export.xlsx');
  });
});
