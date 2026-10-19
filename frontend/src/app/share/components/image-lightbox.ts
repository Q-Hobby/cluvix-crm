import {
  ChangeDetectionStrategy,
  AfterViewInit,
  Component,
  ElementRef,
  EventEmitter,
  HostBinding,
  inject,
  Input,
  OnChanges,
  OnDestroy,
  Output,
  signal,
  SimpleChanges,
  ViewEncapsulation,
} from '@angular/core';
import PhotoSwipeLightbox from 'photoswipe/lightbox';
import 'photoswipe/style.css';

// Kích thước dự phòng khi KHÔNG đo được ảnh (probe lỗi/timeout). PhotoSwipe cần data-pswp-width/height
// để mở; thiếu là nó không mở, và click rơi về điều hướng mặc định của thẻ <a> → văng ra URL gốc.
// Thà mở lightbox với tỉ lệ hơi lệch còn hơn quăng người dùng sang tab CDN.
const PSWP_FALLBACK_W = 1600;
const PSWP_FALLBACK_H = 1200;

// Trần chờ đo ảnh. CDN chậm/chặn hotlink có thể không bao giờ trả onload lẫn onerror.
const PROBE_TIMEOUT_MS = 8000;

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-image-lightbox',
  standalone: true,
  imports: [],
  encapsulation: ViewEncapsulation.None,
  template: `
    @if (src) {
      @if (lightbox) {
        <a [href]="src" (click)="onAnchorClick($event)">
          <img
            [src]="src"
            [alt]="alt || 'Image'"
            [style.width]="imgWidth"
            [style.height]="imgHeight"
            [style.object-fit]="rounded ? 'cover' : 'contain'"
            [class.app-lb-fade]="skeleton"
            [class.app-lb-fade--in]="!skeleton || loaded()"
            (load)="onImgLoaded()"
            (error)="onImgError()"
            loading="lazy"
          />
        </a>
      } @else {
        <img
          [src]="src"
          [alt]="alt || 'Image'"
          [style.width.px]="size"
          [style.height.px]="rounded ? size : 'auto'"
          [style.object-fit]="rounded ? 'cover' : 'contain'"
          [class.app-lb-fade]="skeleton"
          [class.app-lb-fade--in]="!skeleton || loaded()"
          (load)="onImgLoaded()"
          (error)="onImgError()"
          loading="lazy"
        />
      }
      <!-- Skeleton phủ lên ảnh, KHÔNG đẩy layout: khung đã được host giữ chỗ sẵn nên CLS = 0. -->
      @if (skeleton && !loaded()) {
        <span class="app-lb-skeleton" aria-hidden="true"></span>
      }
    } @else {
      <img
        [src]="fallback"
        [alt]="alt || 'No image'"
        [style.width]="imgWidth"
        [style.height]="imgHeight"
        [style.object-fit]="rounded ? 'cover' : 'contain'"
        loading="lazy"
      />
    }
  `,
  styles: [
    `
      /* Hiện ảnh "từ từ" thay vì bật ra đột ngột. Chỉ áp khi bật [skeleton] để các màn đang dùng
         component này giữ nguyên hành vi cũ. */
      .app-lb-fade {
        opacity: 0;
        transition: opacity 220ms ease-out;
      }
      .app-lb-fade--in {
        opacity: 1;
      }
      .app-lb-skeleton {
        position: absolute;
        inset: 0;
        border-radius: inherit;
        background: linear-gradient(
          90deg,
          var(--p-content-hover-background) 25%,
          color-mix(in srgb, var(--p-content-border-color) 45%, var(--p-content-hover-background)) 37%,
          var(--p-content-hover-background) 63%
        );
        background-size: 400% 100%;
        animation: app-lb-shimmer 1.2s ease-in-out infinite;
      }
      @keyframes app-lb-shimmer {
        0% {
          background-position: 100% 0;
        }
        100% {
          background-position: 0 0;
        }
      }
      /* Người dùng bật giảm chuyển động → bỏ shimmer lẫn fade, giữ nền tĩnh. */
      @media (prefers-reduced-motion: reduce) {
        .app-lb-skeleton {
          animation: none;
        }
        .app-lb-fade {
          transition: none;
        }
      }
    `,
  ],
})
export class ImageLightbox implements AfterViewInit, OnChanges, OnDestroy {
  @Input() src?: string;
  @Input() alt?: string;
  @Input() fallback = '/assets/overrides/images/no-image.svg';

  // NEW: tuỳ biến hiển thị
  @Input() lightbox = true; // bật/tắt lightbox
  @Input() size = 96; // px (width only, height auto)
  @Input() rounded = false; // bo tròn (avatar)
  @Input() ring = false; // viền mảnh
  @Input() imgClass = 'w-24 rounded-lg object-cover'; // class cho <img>
  @Input() caption?: string; // optional

  /** Skeleton nhấp nháy trong lúc tải + fade-in khi xong. OPT-IN: mặc định tắt để 10 màn đang dùng
   *  component này không đổi giao diện. Cần đi kèm [aspectRatio] thì mới giữ chỗ được (CLS=0). */
  @Input() skeleton = false;

  /** Tỉ lệ khung giữ chỗ, dạng CSS `aspect-ratio` (vd '4 / 3'). Đặt cùng [skeleton] để khung có
   *  chiều cao NGAY khi chưa biết kích thước ảnh → ảnh load xong không đẩy layout (CLS = 0). */
  @Input() aspectRatio?: string;

  /** Bắn khi ảnh đã render xong (hoặc lỗi) — nơi gọi dùng để cuộn bám đáy sau khi chiều cao đổi. */
  @Output() imgLoaded = new EventEmitter<void>();

  protected loaded = signal(false);

  // Kích thước thẻ <img>.
  //
  // Có [aspectRatio] (khung đã được host giữ chỗ) → ảnh phải LẤP ĐẦY khung rồi `object-fit: contain`
  // co vào cho vừa. Nếu để `height: auto` như đường mặc định thì ảnh giữ tỉ lệ gốc, cao hơn khung, và
  // `overflow: hidden` của host CẮT CỤT phần dưới — ảnh chụp dọc (rất phổ biến trong chat: chụp màn
  // hình, ảnh toa thuốc) chỉ còn thấy nửa trên. Đó là lỗi bản skeleton đầu tiên mắc phải.
  //
  // Không có [aspectRatio] → giữ NGUYÊN hành vi cũ (rộng cố định, cao tự động) cho các màn khác.
  protected get imgWidth(): string {
    return this.aspectRatio && !this.rounded ? '100%' : `${this.size}px`;
  }

  protected get imgHeight(): string {
    if (this.rounded) return `${this.size}px`;
    return this.aspectRatio ? '100%' : 'auto';
  }

  private lb?: PhotoSwipeLightbox;
  private probeTimer?: ReturnType<typeof setTimeout>;
  private loadedTimer?: ReturnType<typeof setTimeout>;
  private initializedFor?: string;
  private setupInFlight = false;
  private el = inject(ElementRef<HTMLElement>);

  @HostBinding('style.width.px') get hostWidth() {
    return this.size;
  }

  @HostBinding('style.height.px') get hostHeight() {
    // Có aspectRatio → để chiều cao cho aspect-ratio quyết định, KHÔNG ép px (ép là hỏng tỉ lệ).
    return this.rounded ? this.size : (this.aspectRatio ? null : 'auto');
  }

  @HostBinding('style.aspect-ratio') get hostAspectRatio() {
    return this.rounded ? null : (this.aspectRatio ?? null);
  }

  @HostBinding('style.border-radius') get hostBorderRadius() {
    return this.rounded ? '50%' : '8px';
  }

  @HostBinding('style.overflow') get hostOverflow() {
    return 'hidden';
  }

  @HostBinding('style.line-height') get hostLineHeight() {
    return '0';
  }

  @HostBinding('class') get hostClass() {
    return [
      'lightgallery-container',
      'relative inline-block',
      this.ring ? 'ring-1 ring-black/5' : '',
    ].join(' ');
  }

  ngAfterViewInit(): void {
    this.setupLightbox();
    this.ensureLoadedFallback();
  }

  // VÌ SAO CẦN OnChanges (bug production 2026-08-15): `src` thường CHƯA có lúc ngAfterViewInit —
  // omnichat nhận tin qua SSE, attachment lúc đó còn status=pending và URL về sau (mirror S3 xong).
  // Bản cũ chỉ init trong ngAfterViewInit và `return` sớm khi thiếu src ⇒ KHÔNG BAO GIỜ init lại ⇒
  // thẻ <a> còn nguyên nên click chạy hành vi mặc định: điều hướng thẳng ra URL cloud.
  // Triệu chứng đặc trưng: ảnh cũ (đã có src lúc reload) bấm ra lightbox bình thường, ảnh vừa nhận
  // thì văng ra CDN.
  ngOnChanges(changes: SimpleChanges): void {
    if (!changes['src']) return;
    this.loaded.set(false);
    this.teardownLightbox();
    // Chờ template đổi href xong rồi mới gắn PhotoSwipe vào anchor mới.
    queueMicrotask(() => {
      this.setupLightbox();
      this.ensureLoadedFallback();
    });
  }

  protected onImgLoaded(): void {
    clearTimeout(this.loadedTimer);
    this.loaded.set(true);
    this.imgLoaded.emit();
  }

  protected onImgError(): void {
    // Ảnh hỏng vẫn phải tắt skeleton, không thì quay mãi.
    this.loaded.set(true);
    this.imgLoaded.emit();
  }

  // Lưới an toàn cuối: PhotoSwipe chưa sẵn sàng thì CHẶN điều hướng thay vì để trình duyệt mở URL
  // gốc. Ở omnichat, URL đó là link CDN Zalo — đá người trực ra khỏi khung chat giữa lúc đang tư vấn.
  protected onAnchorClick(ev: Event): void {
    if (!this.lb) ev.preventDefault();
  }

  private setupLightbox(): void {
    if (!this.lightbox || !this.src) return;
    // ⚠ Guard PHẢI tính cả lượt đang chạy dở. Angular gọi ngOnChanges TRƯỚC ngAfterViewInit, nên lần
    // render đầu có 2 lời gọi setup: một từ ngAfterViewInit, một từ microtask của ngOnChanges. Trong
    // khoảng chờ probe ảnh (async) thì `this.lb` vẫn undefined ⇒ nếu chỉ check `this.lb` thì lượt
    // thứ hai lọt qua, tạo 2 PhotoSwipeLightbox trên cùng gallery: cái đầu rò (vẫn bind click,
    // ngOnDestroy không với tới) và có thể mở chồng 2 overlay.
    if (this.setupInFlight || (this.initializedFor === this.src && this.lb)) return;

    const anchor = this.el.nativeElement.querySelector('a');
    if (!anchor) return;

    const srcAtStart = this.src;
    this.setupInFlight = true;
    let done = false;
    const init = (w: number, h: number) => {
      if (done) return;
      done = true;
      this.setupInFlight = false;
      clearTimeout(this.probeTimer);
      // Chốt chặn thứ hai (phòng đường đi khác trong tương lai): đã có lightbox thì không dựng thêm.
      if (this.lb) return;
      // src đã đổi trong lúc đo → kết quả này lỗi thời, bỏ (lượt setup mới sẽ lo).
      if (this.src !== srcAtStart) return;
      anchor.setAttribute('data-pswp-width', String(w));
      anchor.setAttribute('data-pswp-height', String(h));
      if (this.caption) {
        anchor.setAttribute('title', this.caption);
      }
      this.lb = new PhotoSwipeLightbox({
        gallery: this.el.nativeElement,
        children: 'a',
        pswpModule: () => import('photoswipe'),
      });
      this.lb.init();
      this.initializedFor = srcAtStart;
    };

    const img = new Image();
    img.onload = () => init(img.naturalWidth || PSWP_FALLBACK_W, img.naturalHeight || PSWP_FALLBACK_H);
    // onerror/timeout VẪN init: mất tỉ lệ thật, nhưng giữ được lightbox. Bản cũ bỏ mặc cả hai nhánh
    // này nên hỏng hoàn toàn im lặng.
    img.onerror = () => init(PSWP_FALLBACK_W, PSWP_FALLBACK_H);
    this.probeTimer = setTimeout(() => init(PSWP_FALLBACK_W, PSWP_FALLBACK_H), PROBE_TIMEOUT_MS);
    img.src = srcAtStart;
  }

  private teardownLightbox(): void {
    clearTimeout(this.probeTimer);
    this.setupInFlight = false;
    this.lb?.destroy();
    this.lb = undefined;
    this.initializedFor = undefined;
  }

  // Lưới cho skeleton: ảnh phải HIỆN cho bằng được.
  //
  // `loaded` chỉ bật bởi (load)/(error) ⇒ nếu vì lý do nào đó sự kiện không tới (ảnh đã nằm sẵn
  // trong cache và listener gắn trễ, element bị thay giữa chừng…), ảnh giữ `opacity: 0` VĨNH VIỄN —
  // tức tải về rồi mà người dùng không nhìn thấy gì ngoài shimmer quay mãi. Đó là chế độ hỏng NẶNG
  // HƠN lúc chưa có skeleton, nên phải có 2 đường thoát:
  //   1. `img.complete` — ảnh đã sẵn sàng ngay lúc gắn vào DOM (cache), không đợi sự kiện nữa.
  //   2. timeout trần — chịu thua việc đo, cứ hiện ảnh ra.
  private ensureLoadedFallback(): void {
    if (!this.skeleton || this.loaded()) return;
    const img = this.el.nativeElement.querySelector('img');
    if (img?.complete && img.naturalWidth > 0) {
      this.onImgLoaded();
      return;
    }
    clearTimeout(this.loadedTimer);
    this.loadedTimer = setTimeout(() => {
      if (!this.loaded()) this.onImgLoaded();
    }, PROBE_TIMEOUT_MS);
  }

  ngOnDestroy(): void {
    clearTimeout(this.loadedTimer);
    this.teardownLightbox();
  }
}
