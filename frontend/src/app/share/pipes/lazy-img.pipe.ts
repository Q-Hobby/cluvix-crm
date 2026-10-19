import { Pipe, PipeTransform } from '@angular/core';

/**
 * Inject loading="lazy" decoding="async" vào tất cả <img> tags trong HTML string.
 * Dùng trong list view để tránh browser load đồng thời quá nhiều ảnh khi scroll.
 * Không dùng trong print templates (lazy loading chặn ảnh trước khi in).
 */
@Pipe({
  standalone: true,
  name: 'lazyImg',
  pure: true,
})
export class LazyImgPipe implements PipeTransform {
  transform(html: string | null | undefined): string {
    if (!html) return html ?? '';
    return html.replace(/<img\b(?![^>]*\bloading\s*=)/gi, '<img loading="lazy" decoding="async"');
  }
}
