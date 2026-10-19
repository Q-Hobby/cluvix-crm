import { Service } from '@angular/core';
import type PhotoSwipe from 'photoswipe';
import type { SlideData } from 'photoswipe';

// Kích thước dự phòng khi KHÔNG đo được ảnh (lỗi/timeout). PhotoSwipe cần width/height để dựng khung;
// thà mở với tỉ lệ hơi lệch còn hơn không mở được (trình duyệt văng ra URL ảnh gốc, không có nút đóng).
const FALLBACK_W = 1600;
const FALLBACK_H = 1200;

// Trần chờ đo ảnh — CDN chậm/chặn hotlink có thể không bao giờ trả onload lẫn onerror.
const PROBE_TIMEOUT_MS = 8000;

export interface ImageViewerItem {
  src: string;
  width?: number;
  height?: number;
  alt?: string;
  /** Thumbnail trên trang — PhotoSwipe dùng làm điểm zoom mở/đóng. */
  element?: HTMLElement;
}

/**
 * Trình xem ảnh fullscreen DUY NHẤT cho toàn app (PhotoSwipe).
 *
 * Vì sao cần: trước đây mỗi màn tự dựng viewer riêng (`p-galleria` fullscreen, directive dựng
 * PhotoSwipeLightbox một lần lúc init). Ảnh lớn thì tràn màn hình, nút đóng bị đẩy ra ngoài, hoặc
 * lightbox chưa kịp gắn nên click mở thẳng ảnh gốc — người dùng bị kẹt. PhotoSwipe luôn fit ảnh vào
 * viewport, có nút ✕, Esc, bấm nền/vuốt dọc để đóng, pinch-zoom.
 */
@Service()
export class ImageViewerService {
  private opening = false;
  private current?: PhotoSwipe;

  async open(items: ImageViewerItem[], index = 0): Promise<void> {
    const list = items.filter((x) => !!x.src);
    if (!list.length || this.opening || this.current) return;
    const startIndex = Math.min(Math.max(index, 0), list.length - 1);

    this.opening = true;
    try {
      const dataSource: SlideData[] = list.map((x) => {
        const known = x.width && x.height ? { width: x.width, height: x.height } : this.knownSize(x);
        return { src: x.src, alt: x.alt, element: x.element, width: known?.width, height: known?.height };
      });

      // Chỉ chờ đo ảnh được bấm (thường đã có trong cache vì thumbnail cùng URL). Các ảnh khác đo
      // lười khi lướt tới — chờ đo cả album (hàng chục ảnh gốc) là treo vài giây trước khi mở.
      await this.ensureSize(dataSource[startIndex]);

      const { default: PhotoSwipeCtor } = await import('photoswipe');
      const pswp = new PhotoSwipeCtor({
        dataSource,
        index: startIndex,
        bgOpacity: 0.9,
        showHideAnimationType: 'fade',
        wheelToZoom: true,
        // Nền tối bấm là đóng — đường thoát hiển nhiên nhất khi người dùng không thấy nút ✕.
        bgClickAction: 'close',
      });

      // Đo ảnh hiện tại + 2 ảnh kề (PhotoSwipe dựng sẵn 3 slide) mỗi lần lướt.
      const fixAround = (i: number) => {
        this.fixSizeLazily(pswp, i);
        this.fixSizeLazily(pswp, pswp.getLoopedIndex(i + 1));
        this.fixSizeLazily(pswp, pswp.getLoopedIndex(i - 1));
      };
      pswp.on('change', () => fixAround(pswp.currIndex));

      // Esc: PhotoSwipe nghe keydown ở document, nhưng dialog/drawer PrimeNG phía sau cũng nghe Esc
      // ở document ⇒ một phím đóng luôn cả dialog chứa ảnh. Chặn ở pha capture của window (chạy
      // trước mọi listener ở document) để Esc chỉ đóng viewer.
      const onKeydown = (ev: KeyboardEvent) => {
        if (ev.key !== 'Escape') return;
        ev.preventDefault();
        ev.stopImmediatePropagation();
        pswp.close();
      };
      window.addEventListener('keydown', onKeydown, true);

      pswp.on('destroy', () => {
        window.removeEventListener('keydown', onKeydown, true);
        if (this.current === pswp) this.current = undefined;
      });

      this.current = pswp;
      pswp.init();
      fixAround(startIndex);
    } finally {
      this.opening = false;
    }
  }

  /** Lấy kích thước thật từ thumbnail đang hiển thị nếu nó cùng URL và đã tải xong — không tốn request. */
  private knownSize(item: ImageViewerItem): { width: number; height: number } | undefined {
    const img = item.element instanceof HTMLImageElement ? item.element : item.element?.querySelector('img');
    if (img?.complete && img.naturalWidth > 0 && img.currentSrc === new URL(item.src, location.href).href) {
      return { width: img.naturalWidth, height: img.naturalHeight };
    }
    return undefined;
  }

  private async ensureSize(data: SlideData): Promise<void> {
    if (data.width && data.height) return;
    const size = await this.probe(data.src!);
    data.width = size.width;
    data.height = size.height;
  }

  private fixSizeLazily(pswp: PhotoSwipe, index: number): void {
    const data = (pswp.options.dataSource as SlideData[] | undefined)?.[index];
    if (!data || (data.width && data.height) || data._probing) return;
    data._probing = true;
    void this.probe(data.src!).then((size) => {
      data.width = size.width;
      data.height = size.height;
      data._probing = false;
      // Viewer đã đóng trong lúc đo → không còn gì để vẽ lại.
      if (this.current === pswp) pswp.refreshSlideContent(index);
    });
  }

  private probe(src: string): Promise<{ width: number; height: number }> {
    return new Promise((resolve) => {
      let done = false;
      const finish = (width: number, height: number) => {
        if (done) return;
        done = true;
        clearTimeout(timer);
        resolve({ width: width || FALLBACK_W, height: height || FALLBACK_H });
      };
      const img = new Image();
      img.onload = () => finish(img.naturalWidth, img.naturalHeight);
      // onerror/timeout VẪN resolve: PhotoSwipe tự hiện trạng thái lỗi trong khung, người dùng vẫn đóng được.
      img.onerror = () => finish(FALLBACK_W, FALLBACK_H);
      const timer = setTimeout(() => finish(FALLBACK_W, FALLBACK_H), PROBE_TIMEOUT_MS);
      img.src = src;
    });
  }
}
