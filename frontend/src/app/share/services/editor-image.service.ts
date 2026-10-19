// editor-image.service.ts
import { inject, Service} from '@angular/core';
import { TranslateService } from '@ngx-translate/core';
import { from, Observable, of, throwError } from 'rxjs';
import { catchError, map, mergeMap, switchMap, tap, toArray } from 'rxjs/operators';

import { ImageUploadService } from '@/share/services/image-upload.service';
import { NotificationService } from '@/share/services/notification.service';

export interface ImageProcessResult {
  html: string;
  failed: number;
  total: number;
}

@Service()
export class EditorImageService {
  MAX_IMAGE_SIZE = 10 * 1024 * 1024; // 10MB
  private imageUploadService = inject(ImageUploadService);
  private noti = inject(NotificationService);
  private translate = inject(TranslateService);

  /** 👉 Public API cho NoteForm — có gallery/lightbox structure */
  processNoteHtml(html: string): Observable<ImageProcessResult> {
    return this.replaceBase64ImagesWithLinks(html).pipe(
      map(({ html, failed, total }) => ({
        html: this.convertImagesToGalleryStructure(html),
        failed,
        total,
      })),
    );
  }

  /**
   * 👉 Public API dùng chung cho MỌI `p-editor` khác (appointment, service, faq, zalo post...).
   * Thay base64 -> URL S3, KHÔNG wrap gallery (đặc thù riêng của Note).
   * BẮT BUỘC gọi trước khi gửi payload lên BE cho bất kỳ field HTML nào cho phép dán ảnh —
   * cột DB chỉ TEXT/MEDIUMTEXT, base64 nhúng thẳng dễ vượt giới hạn hoặc phình payload vô ích.
   */
  replaceBase64ImagesInHtml(html: string): Observable<ImageProcessResult> {
    return this.replaceBase64ImagesWithLinks(html);
  }

  /** Thay base64 -> URL; ảnh >10MB hoặc upload lỗi sẽ bị LOẠI khỏi DOM (không để base64 lọt xuống). */
  private replaceBase64ImagesWithLinks(html: string): Observable<ImageProcessResult> {
    const doc = new DOMParser().parseFromString(html || '', 'text/html');
    const imgs = Array.from(doc.body.querySelectorAll('img'));
    const base64Imgs = imgs.filter((it) =>
      (it.getAttribute('src') || '').startsWith('data:image/'),
    );

    if (base64Imgs.length === 0) {
      return of({ html: doc.body.innerHTML, failed: 0, total: 0 });
    }

    let failed = 0;

    const tasks$ = base64Imgs.map((img, index) => {
      const dataUrl = img.getAttribute('src') as string;

      return from(fetch(dataUrl).then((res) => res.blob())).pipe(
        switchMap((blob: Blob) => {
          // chặn >10MB
          if (blob.size > this.MAX_IMAGE_SIZE) {
            failed++;
            this.noti.warnMsg(this.translate.instant('common.message.image_too_large_removed'));
            img.remove();
            return of(null);
          }

          const ext = (blob.type.split('/')[1] || 'png').replace('jpeg', 'jpg');
          const blobInfo = {
            blob: () => blob,
            filename: () => `image_${Date.now()}_${index}.${ext}`,
          };

          return this.imageUploadService.uploadImage(blobInfo).pipe(
            tap((url: string) => {
              if (url) img.setAttribute('src', url);
              else {
                failed++;
                img.remove();
              }
            }),
            catchError((err) => {
              console.error('Upload failed:', err);
              failed++;
              img.remove();
              return of(null); // không văng lỗi; ảnh lỗi đã bị remove
            }),
          );
        }),
      );
    });

    // mergeMap(3) — tối đa 3 upload song song, tránh overload server + browser limit
    return from(tasks$).pipe(
      mergeMap((task$) => task$, 3),
      toArray(),
      map(() => ({ html: doc.body.innerHTML, failed, total: base64Imgs.length })),
      catchError((err) => {
        // sự cố “toàn cục” -> văng lỗi để onSubmit bắt và dừng
        return throwError(() => err);
      }),
    );
  }

  private convertImagesToGalleryStructure(html: string): string {
    // DOMParser (document inert) thay cho `div.innerHTML = html` trên document đang chạy: element
    // tách rời vẫn tải ảnh và bắn `onerror` của HTML người dùng nhập ⇒ XSS (S-5).
    const container = new DOMParser().parseFromString(html || '', 'text/html').body;

    const imgElements = Array.from(container.querySelectorAll('img'));
    const srcList: string[] = [];
    const styleList: string[] = [];

    imgElements.forEach((img: any, index: number) => {
      const src = img.getAttribute('src') || '';
      const style = img.getAttribute('style') || '';
      const placeholder = `{findIMG${index + 1}}`;
      srcList[index + 1] = src;
      styleList[index + 1] = style;
      img.outerHTML = placeholder;
    });

    let updatedHtml = container.innerHTML;

    for (let j = 1; j <= imgElements.length; j++) {
      const src = srcList[j];
      if (!src) {
        updatedHtml = updatedHtml.replace(`{findIMG${j}}`, '');
        continue;
      }
      // src/style lấy từ getAttribute (đã giải mã entity) ⇒ escape lại trước khi ghép vào thuộc tính,
      // không thì một dấu `"` trong giá trị phá thuộc tính và chèn được attribute mới (vd `onerror`).
      const srcAttr = escapeHtmlAttr(src);
      const structure = `
        <div id="anchor-tag-${j}" class="lightgallery-container">
          <a href="${srcAttr}" class="inline-block w-fit max-w-full overflow-hidden">
            <img width="100" src="${srcAttr}" style="${escapeHtmlAttr(styleList[j] || '')}" class="block w-24 h-24 object-cover rounded" loading="lazy" decoding="async" />
          </a>
        </div>
      `;
      updatedHtml = updatedHtml.replace(`{findIMG${j}}`, structure);
    }
    return updatedHtml;
  }
}

/** Escape giá trị để đặt an toàn trong thuộc tính HTML bọc nháy kép. */
function escapeHtmlAttr(value: string): string {
  return value.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}
