import {
  ApplicationRef, ComponentRef, createComponent, DestroyRef, Directive, ElementRef, EnvironmentInjector, inject, Injectable,
  InjectionToken, Injector, input, OnDestroy, OnInit, Type,
} from '@angular/core';

/** Component được đặt vào chỗ — mặc định `DashboardWidgetHost` (Dashboard tự provide), spec thay bằng component giả. */
export const WIDGET_VIEW_POOL_COMPONENT = new InjectionToken<Type<unknown>>('WIDGET_VIEW_POOL_COMPONENT');

/**
 * Giữ SỐNG view của từng khối khi nó đổi chỗ giữa 3 vùng (story-10 AC2, architecture §5 "component được giữ nguyên,
 * chỉ đổi vị trí, nên không mount lại").
 *
 * Vì sao cần: mỗi vùng là một `@for` riêng ⇒ khối sang vùng khác = huỷ ở `@for` cũ + tạo ở `@for` mới ⇒ khối mount lại,
 * tải lại số (bộ đệm SWR vẫn làm mới nền ⇒ gọi lại API), biểu đồ vẽ lại. Pool tạo view khối MỘT lần theo `key`, gắn vào
 * `ApplicationRef` (vẫn được change detection); `[appWidgetSlot]` chỉ DI CHUYỂN node DOM của khối vào phần tử của mình.
 *
 * KHÔNG dùng `ViewContainerRef.insert` ở chỗ đặt: huỷ view `@for` cũ thì Angular huỷ view con trong container của nó
 * TRƯỚC khi gọi `ngOnDestroy` của directive ⇒ chưa kịp tháo đã mất; mà vùng `main` render trước `side` nên kéo
 * main → side luôn rơi vào ca đó.
 *
 * View không còn chỗ nào giữ sau lượt render hiện tại (khối bị ẩn) ⇒ huỷ ở microtask kế: lượt CD là đồng bộ nên mọi
 * `@for` đã chạy xong — khối chỉ đổi vùng thì chỗ mới đã nhận nó. Huỷ khối ẩn để nó không giữ subscription/gọi API khi
 * đổi bộ lọc.
 *
 * PROVIDE ở component Dashboard: `inject(Injector)` khi đó là injector của Dashboard ⇒ khối inject được các store
 * provide ở đó (bộ lọc, bố cục) y như khi khai thẳng trong template.
 */
@Injectable()
export class WidgetViewPool {
  private readonly env = inject(EnvironmentInjector);
  private readonly appRef = inject(ApplicationRef);
  private readonly injector = inject(Injector);
  private readonly component = inject(WIDGET_VIEW_POOL_COMPONENT);
  private readonly refs = new Map<string, ComponentRef<unknown>>();
  private readonly owners = new Map<string, object>();

  constructor() {
    inject(DestroyRef).onDestroy(() => {
      for (const ref of this.refs.values()) ref.destroy();
      this.refs.clear();
      this.owners.clear();
    });
  }

  /** Nhận khối `key` về `host` (tạo nếu chưa có). Gắn sau chủ cũ ⇒ node rời chỗ cũ sang đây, view giữ nguyên. */
  attach(key: string, owner: object, host: HTMLElement): ComponentRef<unknown> {
    let ref = this.refs.get(key);
    if (!ref) {
      ref = createComponent(this.component, { environmentInjector: this.env, elementInjector: this.injector });
      ref.setInput('key', key);
      this.appRef.attachView(ref.hostView);
      this.refs.set(key, ref);
    }
    this.owners.set(key, owner);
    host.appendChild(ref.location.nativeElement as Node);
    return ref;
  }

  /** Số view đang sống — chỉ để spec đối chiếu. */
  get size(): number {
    return this.refs.size;
  }

  release(key: string, owner: object): void {
    if (this.owners.get(key) !== owner) return; // chỗ mới đã nhận khối trước khi chỗ cũ bị huỷ
    this.owners.delete(key);
    queueMicrotask(() => {
      if (this.owners.has(key)) return;
      this.refs.get(key)?.destroy(); // tự tháo khỏi ApplicationRef + gỡ node
      this.refs.delete(key);
    });
  }
}

/** Chỗ đặt khối `key` trong 1 vùng — xem `WidgetViewPool`. */
@Directive({ selector: '[appWidgetSlot]' })
export class WidgetSlotDirective implements OnInit, OnDestroy {
  readonly key = input.required<string>({ alias: 'appWidgetSlot' });
  private readonly pool = inject(WidgetViewPool);
  private readonly el = inject<ElementRef<HTMLElement>>(ElementRef);

  ngOnInit(): void {
    this.pool.attach(this.key(), this, this.el.nativeElement);
  }

  ngOnDestroy(): void {
    // KHÔNG gỡ node: phần tử của chỗ này bị gỡ khỏi DOM kéo theo nó; chỗ mới (nếu có) đã/ sẽ lấy node đi.
    this.pool.release(this.key(), this);
  }
}
