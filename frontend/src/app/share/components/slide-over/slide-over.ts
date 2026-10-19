import { DOCUMENT } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  ElementRef,
  HostListener,
  computed,
  contentChild,
  effect,
  inject,
  input,
  model,
  output,
  signal,
  untracked,
  viewChild,
} from '@angular/core';
import { TranslatePipe } from '@ngx-translate/core';
import { ButtonDirective } from 'primeng/button';
import { FocusTrap } from 'primeng/focustrap';
import { ProgressSpinner } from 'primeng/progressspinner';
import { ZIndexUtils } from 'primeng/utils';

import { SlideOverFooterDirective } from './slide-over-slots';

export type SlideOverSize = 'sm' | 'md' | 'lg' | 'xl' | 'full';
/**
 * `tinted` (mặc định): nền primary rất nhạt — giữ được chất brand mà nút hành động vẫn dùng màu chuẩn.
 * `surface`: nền trắng/tối trơn. `primary`: nền primary đặc (chỉ dùng khi panel không có nút hành động
 * nào trên header, vì nút trên nền đặc buộc phải đổi sang severity nghịch).
 */
export type SlideOverHeaderVariant = 'tinted' | 'primary' | 'surface';

/** Trần chờ animation đóng (ms) — dài hơn animation 300ms một chút. */
const CLOSE_FALLBACK_MS = 450;

/** Chiều rộng panel theo size — dưới breakpoint sm luôn full màn hình. */
const SIZE_CLASS: Record<SlideOverSize, string> = {
  sm: 'sm:w-96',
  md: 'sm:w-[28.6rem]',
  lg: 'sm:w-[38rem]',
  xl: 'sm:w-[52rem]',
  full: 'sm:w-full',
};

/**
 * SlideOver — panel xổ từ mép phải, dùng làm khung chung cho các màn "chi tiết bản ghi"
 * (chi tiết lịch hẹn, đơn hàng, thanh toán...).
 *
 * Khác `p-dialog`/DynamicDialog: SlideOver khai báo ngay trong template của trang gọi nó,
 * mở/đóng bằng signal → data-binding thẳng, không phải truyền model qua `dialogConfig.data`.
 *
 * @example
 * ```html
 * <app-slide-over
 *   [(visible)]="detailVisible"
 *   [title]="'common.label.appointment_detail' | translate"
 *   subtitle="#{{ model()?.code }}"
 *   icon="pi pi-calendar"
 *   size="md"
 *   [loading]="loading()"
 *   (closed)="onDetailClosed()"
 * >
 *   <button slideOverActions pButton severity="contrast" size="small" (click)="createTicket()">
 *     <span pButtonLabel>{{ 'common.action.create' | translate }}</span>
 *   </button>
 *
 *   <app-slide-over-section label="common.label.customer_info"> ... </app-slide-over-section>
 *   <app-slide-over-section label="crm.treatment_service.detail.section_finance"> ... </app-slide-over-section>
 *
 *   <app-dialog-footer slideOverFooter [isSubmitting]="saving()" (cancelForm)="detailVisible.set(false)" (saveForm)="save()" />
 * </app-slide-over>
 * ```
 *
 * ⚠ Overlay của control bên trong (p-select, p-datePicker, p-multiSelect) phải để
 * `appendTo="body"`, nếu render inline sẽ bị vùng body cuộn của panel cắt mất.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-slide-over',
  standalone: true,
  imports: [ButtonDirective, FocusTrap, ProgressSpinner, TranslatePipe],
  template: `
    @if (rendered()) {
      <div #root class="fixed inset-0" role="presentation">
        <!--
          Mask fade riêng nhưng PHẢI cùng thời lượng với panel và PHẢI có animate-fill-forwards:
          animation của tailwindcss-primeui không set animation-fill-mode, hết animation là style
          bật lại giá trị gốc. Mask 0.15s + panel 0.3s ⇒ ở mốc 150ms nền mờ **hiện lại nguyên độ đậm**
          rồi 150ms sau mới biến mất cùng panel — đúng vệt "nhấp nháy" khi bấm đóng.
        -->
        <div
          class="absolute inset-0 bg-black/40 animate-duration-[300ms] animate-fill-forwards"
          [class.animate-fadein]="!closing()"
          [class.animate-fadeout]="closing()"
          (click)="onMaskClick()"
        ></div>

        <section
          #panel
          data-slide-over-panel="1"
          pFocusTrap
          tabindex="-1"
          role="dialog"
          aria-modal="true"
          [attr.aria-label]="title()"
          class="absolute top-0 right-0 h-full w-full shadow-xl outline-none flex flex-col bg-surface-0 dark:bg-surface-900 animate-duration-[300ms] animate-ease-out animate-fill-forwards"
          [class]="sizeClass()"
          [class.animate-fadeinright]="!closing()"
          [class.animate-fadeoutright]="closing()"
          (animationend)="onAnimationEnd($event)"
        >
          <!--
            Header: nền nhạt, chữ theo màu text thường. KHÔNG dùng nền primary đặc — nền đặc buộc mọi
            nút hành động phải đổi sang severity nghịch (contrast = ĐEN), trông như lỗi và mất luôn ý
            nghĩa màu của nút. Nền nhạt cho phép nút giữ đúng bộ màu chuẩn của hệ.
          -->
          <header
            class="flex items-center justify-between gap-3 px-4 py-3 shrink-0 border-b border-surface"
            [class]="headerClass()"
          >
            <div class="flex items-center gap-2.5 min-w-0">
              @if (icon()) {
                <span
                  class="flex items-center justify-center w-8 h-8 shrink-0 rounded-lg"
                  [class]="iconClass()"
                >
                  <i class="text-sm" [class]="icon()"></i>
                </span>
              }
              <div class="flex flex-col min-w-0">
                <h2 class="m-0 text-base font-semibold leading-tight truncate">{{ title() }}</h2>
                @if (subtitle()) {
                  <p class="m-0 text-xs font-normal leading-tight opacity-70 truncate">
                    {{ subtitle() }}
                  </p>
                }
              </div>
            </div>

            <div class="flex items-center gap-1.5 shrink-0">
              <ng-content select="[slideOverActions]" />
              <button
                type="button"
                pButton
                rounded
                text
                size="small"
                [severity]="headerVariant() === 'primary' ? 'contrast' : 'secondary'"
                [attr.aria-label]="'common.action.close' | translate"
                (click)="close()"
              >
                <i class="pi pi-times"></i>
              </button>
            </div>
          </header>

          <!-- Body: vùng cuộn duy nhất của panel -->
          <div class="flex-1 overflow-y-auto overscroll-contain px-6 py-6 flex flex-col gap-6">
            @if (loading()) {
              <div class="flex-1 flex items-center justify-center py-10">
                <p-progress-spinner strokeWidth="4" [style]="{ width: '2.5rem', height: '2.5rem' }" />
              </div>
            } @else if (errorMessage()) {
              <!-- Nạp dữ liệu hỏng: báo NGAY TRONG panel. Trang nền (danh sách + bộ lọc người
                   dùng đang có) phải còn nguyên — xem SKIP_GLOBAL_404 ở response-handler.interceptor. -->
              <div class="flex-1 flex flex-col items-center justify-center gap-3 py-10 text-center">
                <i class="pi pi-exclamation-triangle text-3xl text-muted-color"></i>
                <p class="text-muted-color m-0">{{ errorMessage() }}</p>
                <button type="button" pButton outlined size="small" (click)="retry.emit()">
                  <i class="pi pi-refresh"></i>
                  <span>{{ 'common.action.retry' | translate }}</span>
                </button>
              </div>
            } @else {
              <ng-content />
            }
          </div>

          <!-- Footer: chỉ render khi trang gọi có khai slot [slideOverFooter] -->
          @if (footerSlot()) {
            <div class="shrink-0 border-t border-surface px-6 py-4">
              <ng-content select="[slideOverFooter]" />
            </div>
          }
        </section>
      </div>
    }
  `,
})
export class SlideOver {
  /** Đóng/mở panel — two-way: `[(visible)]="detailVisible"`. */
  readonly visible = model(false);
  readonly title = input('');
  readonly subtitle = input('');
  /** Class icon PrimeIcons, vd `pi pi-calendar`. */
  readonly icon = input('');
  readonly size = input<SlideOverSize>('md');
  readonly headerVariant = input<SlideOverHeaderVariant>('tinted');
  /** Đang tải dữ liệu → body hiện spinner, không render nội dung. */
  readonly loading = input(false);
  /**
   * Nạp dữ liệu thất bại → body hiện thông báo + nút "Thử lại" thay cho nội dung.
   * Chuỗi đã dịch sẵn (trang gọi tự chọn câu theo mã lỗi: 404 khác lỗi mạng).
   */
  readonly errorMessage = input('');
  /** Click ra ngoài mask có đóng panel không (form đang nhập dở nên để false). */
  readonly dismissableMask = input(true);
  readonly closeOnEscape = input(true);
  /** Khoá cuộn body trang nền khi panel mở. */
  readonly blockScroll = input(true);

  /** Bắn ra SAU khi animation đóng chạy xong (panel đã rời DOM). */
  readonly closed = output<void>();
  /** Người dùng bấm "Thử lại" ở trạng thái lỗi — trang gọi nạp lại chi tiết. */
  readonly retry = output<void>();

  protected readonly footerSlot = contentChild(SlideOverFooterDirective);

  /** Panel còn nằm trong DOM (kể cả đang chạy animation đóng). */
  protected readonly rendered = signal(false);
  protected readonly closing = signal(false);

  protected readonly sizeClass = computed(() => SIZE_CLASS[this.size()]);
  protected readonly headerClass = computed(() => {
    switch (this.headerVariant()) {
      case 'primary':
        return 'bg-primary text-primary-contrast border-transparent';
      case 'surface':
        return 'bg-surface-0 dark:bg-surface-900 text-color';
      default:
        return 'bg-primary/5 dark:bg-primary/10 text-color';
    }
  });

  /** Ô icon cạnh tiêu đề — nền primary nhạt để vẫn nhận ra brand khi header không còn nền đặc. */
  protected readonly iconClass = computed(() =>
    this.headerVariant() === 'primary'
      ? 'bg-white/15 text-primary-contrast'
      : 'bg-primary/15 text-primary',
  );

  private readonly rootEl = viewChild<ElementRef<HTMLElement>>('root');
  private readonly panelEl = viewChild<ElementRef<HTMLElement>>('panel');
  private readonly document = inject(DOCUMENT);
  private readonly destroyRef = inject(DestroyRef);

  /** Element đang focus trước khi mở, để trả focus lại lúc đóng (a11y). */
  private previousActiveElement: HTMLElement | null = null;

  /**
   * Chốt chặn cho animation đóng. Panel rời DOM khi nhận `animationend`, nhưng sự kiện đó KHÔNG chắc
   * chắn bắn: user bật `prefers-reduced-motion`, tab bị throttle, hoặc class animation biến mất sau
   * một lần đổi build. Mất sự kiện = panel kẹt trên màn VÀ `body.overflow-hidden` không được gỡ
   * (cả trang không cuộn được) — nên luôn có hẹn giờ đóng cưỡng bức.
   */
  private closeFallbackTimer: ReturnType<typeof setTimeout> | null = null;

  /** Stack các panel đang mở — ESC chỉ đóng panel trên cùng. */
  private static readonly stack: SlideOver[] = [];
  /** Số panel đang khoá cuộn body — tránh mở khoá sớm khi có panel lồng nhau. */
  private static scrollLockCount = 0;

  constructor() {
    // Vào/ra DOM theo `visible`. Đóng phải qua trạng thái `closing` để chạy hết
    // animation trượt ra rồi mới remove khỏi DOM.
    effect(() => {
      const open = this.visible();
      const inDom = untracked(this.rendered);

      if (open && !inDom) {
        this.previousActiveElement = this.document.activeElement as HTMLElement | null;
        this.closing.set(false);
        this.rendered.set(true);
        SlideOver.stack.push(this);
        this.lockScroll();
      } else if (!open && inDom && !untracked(this.closing)) {
        this.closing.set(true);
        this.armCloseFallback();
      }
    });

    // Panel vừa gắn vào DOM.
    effect(() => {
      const root = this.rootEl()?.nativeElement;
      const panel = this.panelEl()?.nativeElement;
      if (!root || !panel) return;

      // Chuyển overlay ra thẳng <body>. Lý do: SlideOver được khai trong template của TRANG, nên
      // `position: fixed` + z-index của nó bị nhốt trong stacking context của trang — topbar
      // (`.layout-topbar`, fixed, z-index 999) vẫn vẽ đè lên 4rem trên cùng, panel trông như bị hụt
      // một đoạn dưới topbar. Ra body thì panel nằm cùng cấp với topbar/sidebar và phủ full viewport.
      // An toàn với Angular: `DefaultDomRenderer2.removeChild` gọi `oldChild.remove()` và BỎ QUA
      // tham số parent, nên `@if` vẫn gỡ đúng node dù nó đã bị chuyển chỗ.
      if (root.parentElement !== this.document.body) {
        this.document.body.appendChild(root);
      }

      // Cùng thang z-index với overlay PrimeNG (base 1000 > topbar 999) → panel phủ được app chrome,
      // còn dialog/select mở TỪ panel vẫn nằm trên panel.
      ZIndexUtils.set('modal', root, 1000);
      panel.focus({ preventScroll: true });
    });

    this.destroyRef.onDestroy(() => {
      this.clearCloseFallback();
      this.teardown();
    });
  }

  /** Đóng panel (chạy animation). Dùng cho nút X, mask, ESC và cả code gọi ngoài. */
  close(): void {
    if (!this.visible()) return;
    this.visible.set(false);
  }

  protected onMaskClick(): void {
    if (this.dismissableMask()) this.close();
  }

  @HostListener('document:keydown.escape')
  protected onEscape(): void {
    // Chỉ panel trên cùng phản ứng, tránh đóng chồng nhiều lớp bằng 1 phím ESC.
    if (!this.closeOnEscape()) return;
    if (SlideOver.stack.at(-1) !== this) return;
    this.close();
  }

  protected onAnimationEnd(event: AnimationEvent): void {
    // animationend nổi bọt từ mọi phần tử con — chỉ nhận đúng animation của panel.
    if ((event.target as HTMLElement)?.dataset?.['slideOverPanel'] !== '1') return;
    this.finalizeClose();
  }

  /** Hẹn giờ đóng cưỡng bức nếu `animationend` không tới (xem `closeFallbackTimer`). */
  private armCloseFallback(): void {
    this.clearCloseFallback();
    this.closeFallbackTimer = setTimeout(() => this.finalizeClose(), CLOSE_FALLBACK_MS);
  }

  private clearCloseFallback(): void {
    if (this.closeFallbackTimer !== null) {
      clearTimeout(this.closeFallbackTimer);
      this.closeFallbackTimer = null;
    }
  }

  /** Gỡ panel khỏi DOM + trả lại focus/scroll. Idempotent: gọi 2 lần (animation + fallback) vẫn đúng. */
  private finalizeClose(): void {
    if (!this.closing()) return;
    this.clearCloseFallback();

    this.teardown();
    this.closing.set(false);
    this.rendered.set(false);
    this.previousActiveElement?.focus?.({ preventScroll: true });
    this.previousActiveElement = null;
    this.closed.emit();
  }

  /** Trả lại z-index, stack và khoá cuộn — gọi cả khi đóng bình thường lẫn khi destroy. */
  private teardown(): void {
    const root = this.rootEl()?.nativeElement;
    if (root) ZIndexUtils.clear(root);

    const idx = SlideOver.stack.indexOf(this);
    if (idx > -1) {
      SlideOver.stack.splice(idx, 1);
      this.unlockScroll();
    }
  }

  private lockScroll(): void {
    if (!this.blockScroll()) return;
    SlideOver.scrollLockCount++;
    this.document.body.classList.add('overflow-hidden');
  }

  private unlockScroll(): void {
    if (!this.blockScroll()) return;
    SlideOver.scrollLockCount = Math.max(0, SlideOver.scrollLockCount - 1);
    if (SlideOver.scrollLockCount === 0) this.document.body.classList.remove('overflow-hidden');
  }
}
