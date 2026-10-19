/**
 * ⚠ Chạy bằng `npx ng test --include=<file này> --watch=false` (builder AOT). `vitest -c vitest-base.config.ts` KHÔNG chạy
 * được: (1) không dịch decorator trong spec ⇒ "Invalid or unexpected token"; (2) JIT không bind signal input của
 * `[appWidgetSlot]` ⇒ NG0950. Cùng lớp `sync-result.spec.ts`.
 */
import { ChangeDetectionStrategy, Component, inject, Input, OnDestroy, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { describe, expect, it } from 'vitest';

import { WIDGET_VIEW_POOL_COMPONENT, WidgetSlotDirective, WidgetViewPool } from './widget-view-pool';

const log = { destroyed: [] as string[] };
let seq = 0;

@Component({ selector: 'fake-widget', template: `<span class="w">{{ uid }}</span>` })
class FakeWidget implements OnDestroy {
  @Input() key = '';
  readonly uid = ++seq;
  ngOnDestroy(): void {
    log.destroyed.push(this.key);
  }
}

/** 2 vùng như shell: mỗi vùng 1 `@for` riêng. `first` = vùng render TRƯỚC (giống `main` trước `side`). */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [WidgetSlotDirective],
  providers: [WidgetViewPool, { provide: WIDGET_VIEW_POOL_COMPONENT, useValue: FakeWidget }],
  template: `
    <section id="first">@for (k of first(); track k) { <div class="slot" [appWidgetSlot]="k"></div> }</section>
    <section id="second">@for (k of second(); track k) { <div class="slot" [appWidgetSlot]="k"></div> }</section>
  `,
})
class TwoZones {
  readonly first = signal<string[]>(['a', 'b']);
  readonly second = signal<string[]>(['c']);
  readonly pool = inject(WidgetViewPool);
}

const flush = () => new Promise<void>((r) => setTimeout(r, 0));
const uidIn = (root: HTMLElement, zone: string) =>
  Array.from(root.querySelectorAll(`#${zone} .w`)).map((x) => x.textContent?.trim());

describe('WidgetViewPool (story-10 AC2: đổi vùng KHÔNG mount lại)', () => {
  // Reset khi DỰNG (không chỉ afterEach): TestBed huỷ fixture của test trước SAU afterEach của file này.
  async function setup() {
    log.destroyed = [];
    const f = TestBed.createComponent(TwoZones);
    f.detectChanges();
    await flush();
    return f;
  }

  it('dựng mỗi khối 1 view, đặt đúng vùng', async () => {
    const f = await setup();
    expect(f.componentInstance.pool.size).toBe(3);
    expect(uidIn(f.nativeElement, 'first')).toHaveLength(2);
    expect(uidIn(f.nativeElement, 'second')).toHaveLength(1);
  });

  it('vùng RENDER TRƯỚC → vùng sau (ca main → side): cùng instance, không huỷ', async () => {
    const f = await setup();
    const before = f.nativeElement.querySelector('#first .slot:nth-child(1) .w').textContent;
    f.componentInstance.first.set(['b']);
    f.componentInstance.second.set(['c', 'a']);
    f.detectChanges();
    await flush();
    expect(log.destroyed).toEqual([]);
    expect(uidIn(f.nativeElement, 'second')[1]).toBe(before);
    expect(f.componentInstance.pool.size).toBe(3);
  });

  it('vùng sau → vùng RENDER TRƯỚC (ca side → main): cùng instance, không huỷ', async () => {
    const f = await setup();
    const before = f.nativeElement.querySelector('#second .w').textContent;
    f.componentInstance.second.set([]);
    f.componentInstance.first.set(['c', 'a', 'b']);
    f.detectChanges();
    await flush();
    expect(log.destroyed).toEqual([]);
    expect(uidIn(f.nativeElement, 'first')[0]).toBe(before);
    expect(f.nativeElement.querySelector('#second .w')).toBeNull();
  });

  it('đổi thứ tự trong vùng: không huỷ', async () => {
    const f = await setup();
    const ids = uidIn(f.nativeElement, 'first');
    f.componentInstance.first.set(['b', 'a']);
    f.detectChanges();
    await flush();
    expect(log.destroyed).toEqual([]);
    expect(uidIn(f.nativeElement, 'first')).toEqual([...ids].reverse());
  });

  it('ẩn khối (không chỗ nào nhận) ⇒ huỷ view sau lượt render; bật lại ⇒ view MỚI', async () => {
    const f = await setup();
    f.componentInstance.first.set(['b']);
    f.detectChanges();
    await flush();
    expect(log.destroyed).toEqual(['a']);
    expect(f.componentInstance.pool.size).toBe(2);
    f.componentInstance.first.set(['b', 'a']);
    f.detectChanges();
    await flush();
    expect(f.componentInstance.pool.size).toBe(3);
  });

  it('huỷ trang ⇒ huỷ mọi view trong pool', async () => {
    const f = await setup();
    f.destroy();
    expect([...log.destroyed].sort()).toEqual(['a', 'b', 'c']);
  });
});
