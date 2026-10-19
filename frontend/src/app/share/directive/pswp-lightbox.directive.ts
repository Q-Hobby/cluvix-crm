import { Directive, ElementRef, HostListener, inject } from '@angular/core';

import { ImageViewerItem, ImageViewerService } from '@/share/services/image-viewer.service';

/**
 * Gắn lên khối HTML chứa ảnh (thường là nội dung Quill qua `[innerHTML]`) → click ảnh mở viewer
 * fullscreen, lướt được giữa các ảnh trong cùng khối.
 *
 * Bắt click theo kiểu delegation tại thời điểm bấm, KHÔNG quét `<a>` một lần lúc init như bản cũ:
 * - Nội dung thường đổ vào SAU ngAfterViewInit (API trả về, `@if`, đổi `[innerHTML]`) → bản cũ không
 *   gắn được lightbox, click chạy hành vi mặc định của `<a>` = mở ảnh gốc to hơn màn hình, không nút đóng.
 * - Bản cũ chờ MỌI ảnh `onload` (không có onerror) → 1 ảnh lỗi là cả khối không bao giờ mở được.
 * - Ảnh dán thẳng vào editor có thể là `<img>` trần, không bọc `<a>` → bản cũ bỏ qua.
 */
@Directive({
  selector: '[pswpLightbox]',
  standalone: true,
})
export class PswpLightboxDirective {
  private el = inject(ElementRef<HTMLElement>);
  private viewer = inject(ImageViewerService);

  @HostListener('click', ['$event'])
  onClick(ev: MouseEvent): void {
    // Giữ hành vi trình duyệt khi người dùng CỐ Ý mở tab mới (Ctrl/Cmd/Shift/chuột giữa).
    if (ev.defaultPrevented || ev.button !== 0 || ev.metaKey || ev.ctrlKey || ev.shiftKey || ev.altKey) return;

    const target = ev.target as HTMLElement | null;
    const clicked = target?.closest('a, img') as HTMLElement | null;
    if (!clicked || !this.el.nativeElement.contains(clicked)) return;

    const items = this.collectItems();
    const index = items.findIndex((x) => x.element === clicked || x.element?.contains(clicked));
    // Link chữ bình thường (không chứa ảnh) → không phải việc của viewer.
    if (index < 0) return;

    ev.preventDefault();
    ev.stopPropagation();
    void this.viewer.open(items, index);
  }

  private collectItems(): ImageViewerItem[] {
    const host = this.el.nativeElement;
    const items: ImageViewerItem[] = [];
    host.querySelectorAll('a, img').forEach((node: Element) => {
      if (node instanceof HTMLAnchorElement) {
        const img = node.querySelector('img');
        if (!img) return;
        items.push({
          element: node,
          // href là ảnh gốc; thumbnail có thể là bản thu nhỏ. href rỗng → dùng src của thumbnail.
          src: node.getAttribute('href') ? node.href : img.currentSrc || img.src,
          width: Number(node.getAttribute('data-pswp-width')) || undefined,
          height: Number(node.getAttribute('data-pswp-height')) || undefined,
          alt: img.alt,
        });
      } else if (node instanceof HTMLImageElement && !node.closest('a')) {
        items.push({ element: node, src: node.currentSrc || node.src, alt: node.alt });
      }
    });
    return items.filter((x) => !!x.src);
  }
}
