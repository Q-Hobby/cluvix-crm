import { NgComponentOutlet } from '@angular/common';
import { ChangeDetectionStrategy, Component, computed, inject, Injector, input } from '@angular/core';

import { createChunkLoader } from './chunk-loader.util';
import { DashboardLayoutStore } from './dashboard-layout.store';
import { CRM_DASHBOARD_WIDGET_MAP, WIDGET_CATEGORY_ACCENT } from './dashboard-widgets';
import { LazyVisibleDirective } from './lazy-visible.directive';
import { DASHBOARD_WIDGET_CONTEXT, type DashboardWidgetContext } from './widget-data';
import { DashboardWidgetFrame } from './widget-frame';

/**
 * Chỗ đặt 1 khối trong shell (story-09): chỉ tải CHUNK JS của khối khi nó vào gần viewport (appLazyVisible 600px) —
 * khối ẩn không render ⇒ không tải JS, không gọi API (PRD FR2/AC15). Giữ chỗ bằng khung khối ở trạng thái loading;
 * tải chunk lỗi ⇒ khung lỗi + Thử lại (QA P1).
 */
@Component({
  selector: 'app-dashboard-widget-host',
  host: { class: 'block' },
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [NgComponentOutlet, LazyVisibleDirective, DashboardWidgetFrame],
  template: `
    <div class="h-full" (appLazyVisible)="load()">
      @if (type(); as t) {
        <ng-container *ngComponentOutlet="t; injector: outletInjector()" />
      } @else if (def(); as d) {
        <!-- Chưa có chunk / tải chunk lỗi ⇒ CHÍNH khung khối ở trạng thái loading/error: cùng tiêu đề, cùng khung xương
             (chiều cao từ registry) với pha tải số ⇒ không nhảy khung giữa 2 pha; lỗi dùng chung khung lỗi + Thử lại. -->
        <app-dashboard-widget-frame [titleKey]="d.titleKey" [category]="d.category" [skeleton]="d.skeleton" [bodyHeight]="d.chartHeight ?? null"
                                    [status]="failed() ? 'error' : 'loading'" (retry)="load()" />
      }
    </div>
  `,
})
export class DashboardWidgetHost {
  readonly key = input.required<string>();
  protected readonly def = computed(() => CRM_DASHBOARD_WIDGET_MAP.get(this.key()));
  private readonly chunk = createChunkLoader(() => {
    const d = this.def();
    return d ? d.loadComponent() : Promise.reject(new Error(`unknown widget ${this.key()}`));
  });
  protected readonly type = this.chunk.type;

  private readonly store = inject(DashboardLayoutStore, { optional: true });
  private readonly injector = inject(Injector);
  /** Khối inject `DASHBOARD_WIDGET_CONTEXT` ⇒ menu "…" có mục Ẩn (đang sửa: vào nháp; đang xem: lưu luôn). */
  protected readonly outletInjector = computed(() => {
    const store = this.store;
    const key = this.key();
    if (!store) return this.injector;
    const ctx: DashboardWidgetContext = {
      canHide: computed(() => store.canPersist && !store.saving()),
      hide: () => store.hideWidget(key).subscribe({ error: () => undefined }), // lỗi: interceptor đã toast, store đã trả lại
      accent: WIDGET_CATEGORY_ACCENT[this.def()?.category ?? 'revenue'],
      category: this.def()?.category,
    };
    return Injector.create({ providers: [{ provide: DASHBOARD_WIDGET_CONTEXT, useValue: ctx }], parent: this.injector });
  });
  protected readonly failed = this.chunk.failed;

  protected load(): void {
    if (!this.def()) return;
    void this.chunk.load();
  }
}
