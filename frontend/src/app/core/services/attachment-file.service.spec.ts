import { DestroyRef } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TranslateService } from '@ngx-translate/core';
import { environment } from '@env/environment';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  AttachmentFileService,
  AttachmentMediaUrls,
  MEDIA_RETRY_WINDOW_MS,
  resolveSignedUrl,
} from './attachment-file.service';
import { SKIP_GLOBAL_403, SKIP_GLOBAL_404 } from '@/core/interceptors/response-handler.interceptor';
import { NotificationService } from '@/share/services/notification.service';

/** File đính kèm khách là riêng tư: xem/phát/tải qua URL KÝ NGẮN HẠN, không blob, không JWT trong URL. */
describe('AttachmentFileService', () => {
  const api = environment.apiUrl.replace(/\/+$/, '');
  const item = { id: 7, file_name: 'a.mp4' };
  const item2 = { id: 8, file_name: 'b.mp4' };
  const signPath = (id: number) => `${api}/crm/attachment/${id}/signed-url`;
  const signed = (url: string, legacy = false) => ({ data: { url, expires_at: 1_900_000_000, legacy } });

  let http: HttpTestingController;
  let svc: AttachmentFileService;
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
    svc = TestBed.inject(AttachmentFileService);
  });

  afterEach(() => {
    http.verify();
    vi.restoreAllMocks();
  });

  it('getSignedUrl POST đúng endpoint, lỗi cục bộ, ghép /api/files/signed vào apiUrl', () => {
    let got = '';
    svc.getSignedUrl(7).subscribe((s) => (got = s.url));
    const req = http.expectOne(signPath(7));
    expect(req.request.method).toBe('POST');
    expect(req.request.body).toEqual({});
    expect(req.request.context.get(SKIP_GLOBAL_403)).toBe(true);
    expect(req.request.context.get(SKIP_GLOBAL_404)).toBe(true);
    req.flush(signed('/api/files/signed/tok.sig'));
    expect(got).toBe(new URL(`${api}/files/signed/tok.sig`, window.location.origin).href);
  });

  it('resolveSignedUrl giữ URL tuyệt đối http(s), từ chối scheme lạ', () => {
    expect(resolveSignedUrl('https://bucket.example.com/a.mp4?X-Amz-Expires=300')).toBe(
      'https://bucket.example.com/a.mp4?X-Amz-Expires=300',
    );
    expect(() => resolveSignedUrl('javascript:alert(1)')).toThrow();
    expect(() => resolveSignedUrl('data:text/html,x')).toThrow();
    expect(() => resolveSignedUrl('')).toThrow();
  });

  it('download xin URL ký với download=true rồi điều hướng qua <a> (không tải blob)', () => {
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined);
    svc.download(item);
    const req = http.expectOne(signPath(7));
    expect(req.request.body).toEqual({ download: true });
    req.flush(signed('https://s3.example.com/priv/a.mp4?sig=1'));
    expect(click).toHaveBeenCalledTimes(1);
    const a = click.mock.contexts[0] as HTMLAnchorElement;
    expect(a.href).toBe('https://s3.example.com/priv/a.mp4?sig=1');
    expect(a.target).toBe('');
  });

  it('lỗi 403 khi xin URL ⇒ báo toast', () => {
    svc.download(item);
    http.expectOne(signPath(7)).flush({ message: 'x' }, { status: 403, statusText: 'Forbidden' });
    expect(errorMsg).toHaveBeenCalledWith('common.message.attachment_unavailable');
  });

  describe('AttachmentMediaUrls', () => {
    it('xin URL KHI bấm phát (một lần, không trùng khi đang xin)', () => {
      const media = new AttachmentMediaUrls(svc);
      http.expectNone(signPath(7));
      media.load(item);
      media.load(item);
      expect(media.loadingIds().has(7)).toBe(true);
      http.expectOne(signPath(7)).flush(signed('https://s3.example.com/a.mp4?v=1'));
      expect(media.urls()[7]).toBe('https://s3.example.com/a.mp4?v=1');
      expect(media.loadingIds().size).toBe(0);
      media.load(item); // đã có URL ⇒ không xin lại
      http.expectNone(signPath(7));
    });

    it('media lỗi ⇒ xin lại URL đúng 1 lần, tua về vị trí cũ; lỗi tiếp ngay ⇒ bỏ cuộc + toast', () => {
      const media = new AttachmentMediaUrls(svc);
      media.load(item);
      http.expectOne(signPath(7)).flush(signed('https://s3.example.com/a.mp4?v=1'));

      const el = { currentTime: 42 } as HTMLMediaElement;
      media.onMediaError(7, el);
      http.expectOne(signPath(7)).flush(signed('https://s3.example.com/a.mp4?v=2'));
      expect(media.urls()[7]).toBe('https://s3.example.com/a.mp4?v=2');
      const fresh = { currentTime: 0 } as HTMLMediaElement;
      media.onMediaLoaded(7, fresh);
      expect(fresh.currentTime).toBe(42);

      media.onMediaError(7, fresh);
      http.expectNone(signPath(7));
      expect(media.urls()[7]).toBeUndefined();
      expect(errorMsg).toHaveBeenCalledWith('common.message.attachment_unavailable');

      // Bấm phát lại sau khi bỏ cuộc ⇒ xin URL mới như thường.
      media.load(item);
      http.expectOne(signPath(7)).flush(signed('https://s3.example.com/a.mp4?v=3'));
      expect(media.urls()[7]).toBe('https://s3.example.com/a.mp4?v=3');
    });

    it('hết hạn lần nữa sau cửa sổ retry (xem lâu) ⇒ vẫn được xin lại', () => {
      let now = 1_000_000;
      vi.spyOn(Date, 'now').mockImplementation(() => now);
      const media = new AttachmentMediaUrls(svc);
      media.load(item);
      http.expectOne(signPath(7)).flush(signed('https://s3.example.com/a.mp4?v=1'));
      media.onMediaError(7);
      http.expectOne(signPath(7)).flush(signed('https://s3.example.com/a.mp4?v=2'));
      now += MEDIA_RETRY_WINDOW_MS + 1;
      media.onMediaError(7);
      http.expectOne(signPath(7)).flush(signed('https://s3.example.com/a.mp4?v=3'));
      expect(media.urls()[7]).toBe('https://s3.example.com/a.mp4?v=3');
      expect(errorMsg).not.toHaveBeenCalled();
    });

    it('xin URL lỗi ⇒ toast, nút phát hiện lại (không URL)', () => {
      const media = new AttachmentMediaUrls(svc);
      media.load(item);
      http.expectOne(signPath(7)).flush({}, { status: 404, statusText: 'Not Found' });
      expect(media.urls()[7]).toBeUndefined();
      expect(media.loadingIds().size).toBe(0);
      expect(errorMsg).toHaveBeenCalledTimes(1);
    });

    it('destroy GIỮA lúc xin (DestroyRef): huỷ request, load sau đó là no-op', () => {
      let onDestroy: (() => void) | undefined;
      const destroyRef = { onDestroy: (fn: () => void) => { onDestroy = fn; return () => undefined; } } as unknown as DestroyRef;
      const media = new AttachmentMediaUrls(svc, destroyRef);
      media.load(item);
      const req = http.expectOne(signPath(7));
      onDestroy!();
      expect(req.cancelled).toBe(true);
      expect(media.urls()).toEqual({});
      expect(media.loadingIds().size).toBe(0);
      media.load(item2);
      http.expectNone(signPath(8));
    });

    it('reset (đổi khách) huỷ request đang xin + bỏ URL; forget(id) bỏ đúng 1 file', () => {
      const media = new AttachmentMediaUrls(svc);
      media.load(item);
      http.expectOne(signPath(7)).flush(signed('https://s3.example.com/a.mp4'));
      media.load(item2);
      const pending = http.expectOne(signPath(8));
      media.reset();
      expect(pending.cancelled).toBe(true);
      expect(media.urls()).toEqual({});

      media.load(item);
      http.expectOne(signPath(7)).flush(signed('https://s3.example.com/a.mp4'));
      media.load(item2);
      http.expectOne(signPath(8)).flush(signed('https://s3.example.com/b.mp4'));
      media.forget(7);
      expect(media.urls()).toEqual({ 8: 'https://s3.example.com/b.mp4' });
    });
  });
});
