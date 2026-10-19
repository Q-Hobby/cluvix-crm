import { HttpClient, HttpContext, HttpErrorResponse } from '@angular/common/http';
import { DestroyRef, Injectable, inject, signal } from '@angular/core';
import { TranslateService } from '@ngx-translate/core';
import { environment } from '@env/environment';
import { Observable, Subscription, map } from 'rxjs';

import {
  SKIP_ERROR_NOTIFICATION,
  SKIP_GLOBAL_403,
  SKIP_GLOBAL_404,
} from '@/core/interceptors/response-handler.interceptor';
import { NotificationService } from '@/share/services/notification.service';

/** Phần của item đính kèm mà service cần. */
export interface AttachmentRef {
  id: number;
  file_name: string;
}

/** Kết quả POST /crm/attachment/:id/signed-url (url đã được ghép thành URL tuyệt đối dùng được ngay). */
export interface SignedAttachmentUrl {
  url: string;
  /** Unix giây; 0 với bản ghi cũ (legacy — URL công khai không hạn). */
  expires_at: number;
  legacy: boolean;
}

const API_BASE = environment.apiUrl.replace(/\/+$/, '');

/**
 * BE trả URL tuyệt đối (s3/r2 presign, local có STORAGE_PUBLIC_BASE, legacy) hoặc đường dẫn gốc site
 * `/api/files/signed/…` (local, STORAGE_PUBLIC_BASE trống) ⇒ ghép vào `environment.apiUrl` (đã gồm `/api`,
 * có thể khác origin FE). Chỉ nhận http(s) — mọi scheme khác (javascript:, data:) bị từ chối.
 */
export function resolveSignedUrl(url: string): string {
  const raw = (url ?? '').trim();
  let abs: string;
  if (/^https?:\/\//i.test(raw) || raw.startsWith('//')) {
    abs = raw;
  } else if (raw.startsWith('/api/')) {
    abs = API_BASE + raw.slice('/api'.length);
  } else {
    throw new Error('signed url: unsupported');
  }
  const parsed = new URL(abs, window.location.origin);
  if (parsed.protocol !== 'https:' && parsed.protocol !== 'http:') throw new Error('signed url: unsupported');
  return parsed.href;
}

/**
 * File đính kèm khách là RIÊNG TƯ: BE không trả `file_url`. Xem/phát/tải đều qua URL KÝ NGẮN HẠN
 * (POST /crm/attachment/:id/signed-url — JWT + quyền + scope kiểm lúc cấp): trình duyệt đọc thẳng file,
 * có HTTP Range ⇒ video/audio tua được, không phải tải trọn blob vào RAM. KHÔNG đính JWT vào URL.
 *
 * Nút "Tải xuống" cũng dùng URL ký (download=true ⇒ BE ký Content-Disposition: attachment) thay vì tải
 * blob qua XHR — đồng nhất một đường, file lớn không chiếm RAM tab. Bản ghi cũ (legacy) BE trả nguyên URL
 * công khai cũ ⇒ mở tab mới (trước đây XHR theo 302 cross-origin dính CORS).
 */
@Injectable({ providedIn: 'root' })
export class AttachmentFileService {
  private readonly http = inject(HttpClient);
  private readonly noti = inject(NotificationService);
  private readonly translate = inject(TranslateService);

  getSignedUrl(id: number, opts: { download?: boolean } = {}): Observable<SignedAttachmentUrl> {
    // Lỗi là CỤC BỘ — báo tại chỗ, không đá cả trang sang notfound/accessdenied.
    const context = new HttpContext()
      .set(SKIP_GLOBAL_404, true)
      .set(SKIP_GLOBAL_403, true)
      .set(SKIP_ERROR_NOTIFICATION, true);
    return this.http
      .post<{ data?: SignedAttachmentUrl } & Partial<SignedAttachmentUrl>>(
        `${API_BASE}/crm/attachment/${id}/signed-url`,
        opts.download ? { download: true } : {},
        { context },
      )
      .pipe(
        map((res) => {
          const d = (res?.data ?? res) as SignedAttachmentUrl;
          return { url: resolveSignedUrl(d.url), expires_at: Number(d.expires_at) || 0, legacy: !!d.legacy };
        }),
      );
  }

  /** Tải xuống: URL ký với Content-Disposition attachment ⇒ trình duyệt tự tải, trang hiện tại giữ nguyên. */
  download(item: AttachmentRef): void {
    this.getSignedUrl(item.id, { download: true }).subscribe({
      next: (s) => this.navigate(s.url, s.legacy, item.file_name),
      error: (err: HttpErrorResponse) => this.notifyError(err),
    });
  }

  /** Xem trong tab mới (ảnh/PDF…). Mở tab đồng bộ trong thao tác click để không bị chặn popup. */
  open(item: AttachmentRef): void {
    const tab = window.open('', '_blank');
    this.getSignedUrl(item.id).subscribe({
      next: (s) => {
        if (tab) {
          // Cắt liên kết ngược tới trang CRM trước khi điều hướng (tab mở bằng '' giữ opener).
          tab.opener = null;
          tab.location.href = s.url;
        } else {
          this.navigate(s.url, true, item.file_name);
        }
      },
      error: (err: HttpErrorResponse) => {
        tab?.close();
        this.notifyError(err);
      },
    });
  }

  notifyError(err?: HttpErrorResponse | Error | null): void {
    // 401 đã do authInterceptor xử lý (refresh/đăng xuất) — không báo trùng.
    if ((err as HttpErrorResponse | undefined)?.status === 401) return;
    this.noti.errorMsg(this.translate.instant('common.message.attachment_unavailable'));
  }

  private navigate(url: string, newTab: boolean, fileName: string): void {
    const a = document.createElement('a');
    a.href = url;
    a.rel = 'noopener noreferrer';
    // `download` chỉ có tác dụng cùng origin; khác origin thì Content-Disposition (đã ký) quyết định.
    a.download = fileName || '';
    if (newTab) a.target = '_blank';
    document.body.appendChild(a);
    a.click();
    a.remove();
  }
}

/** Khoảng tối thiểu giữa 2 lần xin lại URL cho cùng 1 file khi media lỗi (chặn vòng lặp lỗi thật ⇄ xin lại). */
export const MEDIA_RETRY_WINDOW_MS = 60_000;

/**
 * Giữ URL ký phát video/audio theo id, xin LƯỜI (chỉ khi người dùng bấm phát) để không ký cả loạt khi mở tab.
 * `<video|audio [src] preload="metadata">` phát thẳng từ URL — không blob, không objectURL.
 *
 * URL hết hạn khi đang xem (tua/phát tiếp sau khi quá hạn ⇒ request Range bị 403/410 ⇒ sự kiện `error`
 * của media): `onMediaError` xin URL MỚI một lần, nhớ vị trí đang phát để `onMediaLoaded` tua về. Lỗi lại
 * trong vòng MEDIA_RETRY_WINDOW_MS ⇒ coi là lỗi thật (file hỏng/mất quyền): bỏ URL, báo toast, nút phát
 * hiện lại. Hết hạn lần nữa sau đó (xem rất lâu) vẫn được xin lại.
 *
 * Vòng đời: truyền `DestroyRef` ⇒ tự `dispose()`; `reset()` khi đổi khách (huỷ request đang xin + bỏ URL
 * của khách cũ); `forget(id)` khi xoá 1 file.
 */
export class AttachmentMediaUrls {
  readonly urls = signal<Record<number, string>>({});
  readonly loadingIds = signal<ReadonlySet<number>>(new Set());

  private readonly inflight = new Map<number, Subscription>();
  private readonly retriedAt = new Map<number, number>();
  private readonly resumeAt = new Map<number, number>();
  private disposed = false;

  constructor(
    private readonly svc: AttachmentFileService,
    destroyRef?: DestroyRef,
  ) {
    destroyRef?.onDestroy(() => this.dispose());
  }

  load(item: { id: number }): void {
    if (this.disposed) return;
    if (this.urls()[item.id] || this.loadingIds().has(item.id)) return;
    this.retriedAt.delete(item.id);
    this.request(item.id);
  }

  /** `(error)` của `<video>`/`<audio>`: xin URL mới 1 lần (URL ký có thể đã hết hạn giữa chừng). */
  onMediaError(id: number, el?: HTMLMediaElement | null): void {
    if (this.disposed || !this.urls()[id] || this.loadingIds().has(id)) return;
    const last = this.retriedAt.get(id);
    if (last !== undefined && Date.now() - last < MEDIA_RETRY_WINDOW_MS) {
      this.forget(id);
      this.svc.notifyError();
      return;
    }
    this.retriedAt.set(id, Date.now());
    if (el && el.currentTime > 0) this.resumeAt.set(id, el.currentTime);
    this.request(id);
  }

  /** `(loadedmetadata)`: sau khi xin lại URL ⇒ tua về vị trí đang phát trước lỗi. */
  onMediaLoaded(id: number, el?: HTMLMediaElement | null): void {
    const t = this.resumeAt.get(id);
    this.resumeAt.delete(id);
    if (el && t !== undefined) el.currentTime = t;
  }

  /** Bỏ URL của 1 file (xoá file / bỏ cuộc sau lỗi). */
  forget(id: number): void {
    this.inflight.get(id)?.unsubscribe();
    this.inflight.delete(id);
    this.retriedAt.delete(id);
    this.resumeAt.delete(id);
    this.stopLoading(id);
    if (!(id in this.urls())) return;
    this.urls.update((m) => {
      const { [id]: _drop, ...rest } = m;
      return rest;
    });
  }

  /** Đổi ngữ cảnh (khách khác): huỷ mọi request đang xin + bỏ mọi URL. Dùng tiếp được. */
  reset(): void {
    this.inflight.forEach((sub) => sub.unsubscribe());
    this.inflight.clear();
    this.retriedAt.clear();
    this.resumeAt.clear();
    this.loadingIds.set(new Set());
    this.urls.set({});
  }

  /** Huỷ hẳn (component destroy): reset + chặn mọi `load()`/URL về muộn. */
  dispose(): void {
    this.disposed = true;
    this.reset();
  }

  private request(id: number): void {
    this.loadingIds.update((s) => new Set(s).add(id));
    const sub = this.svc.getSignedUrl(id).subscribe({
      next: (s) => {
        this.inflight.delete(id);
        if (this.disposed) return;
        this.urls.update((m) => ({ ...m, [id]: s.url }));
        this.stopLoading(id);
      },
      error: (err: HttpErrorResponse) => {
        this.inflight.delete(id);
        if (this.disposed) return;
        this.forget(id);
        this.svc.notifyError(err);
      },
    });
    if (!sub.closed) this.inflight.set(id, sub);
  }

  private stopLoading(id: number): void {
    if (!this.loadingIds().has(id)) return;
    this.loadingIds.update((s) => {
      const n = new Set(s);
      n.delete(id);
      return n;
    });
  }
}
