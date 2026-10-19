import { AfterViewInit, Directive, ElementRef, inject, input, OnDestroy, Renderer2 } from '@angular/core';

/**
 * Gắn lên <p-table> (hoặc bất kỳ container nào cần "chiếm hết chỗ còn lại") để tự fill hết không
 * gian còn lại của ancestor flex-bounded gần nhất (thay cho `TableLayoutService`/`calc(100vh - Npx)`).
 *
 * PHẢI tự khai `[scrollHeight]="'flex'"` cùng directive này khi host là `<p-table>` — PrimeNG v22
 * `Table.scrollHeight` là InputSignal (chỉ đọc), directive không gán được từ ngoài. Host không phải
 * `<p-table>` (vd 1 `<div>` bọc cả trang report) thì bỏ qua thuộc tính này.
 *
 * 2 chế độ:
 *
 * - `appFillHeight` (mặc định, `''`) — CHỈ lo phần "chính host này tự fill". Ancestor phải TỰ đảm
 *   bảo `display:flex;flex-direction:column;min-height:0` ở mỗi tầng cha. Dùng cho p-table nhúng
 *   trong trang khác đã có khung flex sẵn (vd tab "Thanh toán"/"Giấy tờ" của `emr/examination`).
 *
 * - `appFillHeight="page"` — trang ROUTED top-level, ancestor là shell dùng chung
 *   (`.layout-content-wrapper > .layout-content`) vốn KHÔNG flex. Directive tự dựng chuỗi flex từ
 *   host lên tới `.layout-content-wrapper` rồi gỡ sạch khi component bị huỷ. Nhờ đó chiều cao khả
 *   dụng được browser tính, KHÔNG phải trừ hằng số theo `window.innerWidth` (hằng số luôn sai:
 *   breadcrumb có thể xuống 2 dòng, menu ngang đổi topbar 4rem→7rem).
 *
 * ```html
 * <p-table appFillHeight="page" [scrollHeight]="'flex'" [style]="{ width: '100%' }">
 * ```
 *
 * ## 2 gotcha đã xử lý khi đi qua ancestor không phải "div trần"
 *
 * - **Ancestor đã tự là flex/grid container** (PrimeFlex `.grid` — `display:flex;flex-wrap:wrap`
 *   dùng cho layout 2 cột CẠNH NHAU, hoặc Tailwind `.grid` — CSS Grid dùng cho layout nhiều cột):
 *   ép thêm `display:flex;flex-direction:column` đè lên sẽ PHÁ layout ngang/lưới đang có (biến 2
 *   cột cạnh nhau thành xếp chồng). Directive đo `getComputedStyle(node).display` TRƯỚC khi quyết
 *   định — ancestor đã là `flex`/`inline-flex`/`grid`/`inline-grid` thì CHỈ thêm class sizing
 *   (`cx-fill-chain`: `flex:1 1 auto;min-height:0`), KHÔNG thêm class ép hướng
 *   (`cx-fill-chain-stack`: `display:flex;flex-direction:column`) — layout gốc của ancestor giữ
 *   nguyên, chỉ chiều cao TOÀN KHỐI được flex hoá.
 * - **Ancestor có thể mang attribute `hidden`** (PrimeNG `<p-tabpanel>` — `[hidden]="!active()"`,
 *   xem `primeng-tabs.mjs`): CSS tác giả (author) LUÔN thắng UA stylesheet với declaration thường
 *   (không `!important`) BẤT KỂ specificity — nếu class `cx-fill-chain(-stack)` set
 *   `display:flex` KHÔNG ĐIỀU KIỆN, tab đã ẩn (`[hidden]`) vẫn hiển thị đè lên tab đang active (bug
 *   thật đã từng phải revert 2026-08-07, xem `bmad/done/p-table-full-height/`). 2 class CSS
 *   (`_content.scss`) đều guard bằng `:not([hidden])` — ancestor đang `[hidden]` thì KHÔNG rule
 *   author nào khớp, UA stylesheet `[hidden]{display:none}` áp dụng bình thường.
 */
@Directive({
  selector: '[appFillHeight]',
  standalone: true,
})
export class FillHeightDirective implements AfterViewInit, OnDestroy {
  /** `''` = chỉ host tự fill; `'page'` = tự dựng cả chuỗi flex của shell dùng chung. */
  readonly appFillHeight = input<'' | 'page'>('');

  private static readonly WRAPPER_CLASS = 'layout-content-wrapper';
  private static readonly PAGE_CLASS = 'cx-fill-page';
  private static readonly CHAIN_CLASS = 'cx-fill-chain';
  private static readonly CHAIN_STACK_CLASS = 'cx-fill-chain-stack';
  private static readonly ALREADY_FLEX_DISPLAY = new Set(['flex', 'inline-flex', 'grid', 'inline-grid']);

  private el = inject(ElementRef);
  private renderer = inject(Renderer2);

  /** Ancestor đã bị directive này gắn class — nhớ lại để ngOnDestroy gỡ đúng những gì đã gắn. */
  private touched: { node: HTMLElement; cls: string }[] = [];

  ngAfterViewInit(): void {
    this.renderer.addClass(this.el.nativeElement, 'flex-1');
    this.renderer.addClass(this.el.nativeElement, 'min-h-0');

    if (this.appFillHeight() === 'page') {
      this.prepareAncestorChain();
    }
  }

  ngOnDestroy(): void {
    // BẮT BUỘC: `.layout-content-wrapper`/`.layout-content` thuộc `AppLayout`, KHÔNG bị tạo lại
    // khi đổi route. Không gỡ ⇒ trang kế tiếp (form dài, dashboard) thừa hưởng khung
    // `height:100vh` và bị cắt cụt nội dung.
    for (const { node, cls } of this.touched) {
      this.renderer.removeClass(node, cls);
    }
    this.touched = [];
  }

  /**
   * Đi ngược từ host lên tới `.layout-content-wrapper`, gắn flex chain. Không tìm thấy wrapper
   * (host nằm ngoài `AppLayout`, vd trang in) ⇒ KHÔNG gắn gì cả, tránh sửa DOM lạ.
   */
  private prepareAncestorChain(): void {
    const chain: HTMLElement[] = [];
    let node: HTMLElement | null = (this.el.nativeElement as HTMLElement).parentElement;

    while (node && node !== document.body) {
      if (node.classList.contains(FillHeightDirective.WRAPPER_CLASS)) {
        for (const item of chain) {
          this.addChainNode(item);
        }
        this.addTracked(node, FillHeightDirective.PAGE_CLASS);
        return;
      }
      chain.push(node);
      node = node.parentElement;
    }
  }

  /** Sizing luôn thêm; ép hướng flex-column CHỈ khi ancestor chưa tự là flex/grid container. */
  private addChainNode(node: HTMLElement): void {
    this.addTracked(node, FillHeightDirective.CHAIN_CLASS);
    if (!FillHeightDirective.ALREADY_FLEX_DISPLAY.has(getComputedStyle(node).display)) {
      this.addTracked(node, FillHeightDirective.CHAIN_STACK_CLASS);
    }
  }

  private addTracked(node: HTMLElement, cls: string): void {
    this.renderer.addClass(node, cls);
    this.touched.push({ node, cls });
  }
}
