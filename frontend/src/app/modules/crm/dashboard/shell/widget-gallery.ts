import { ChangeDetectionStrategy, Component, computed, inject, input, model, output, signal } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { TranslatePipe, TranslateService } from '@ngx-translate/core';
import { Drawer } from 'primeng/drawer';
import { InputTextModule } from 'primeng/inputtext';
import type { Observable } from 'rxjs';

import { normalizeSearchText } from '@/share/utils/text-normalize.util';

import { CRM_DASHBOARD_WIDGETS, type DashboardWidgetDef, WIDGET_CATEGORY_LABEL, type WidgetCategory } from './dashboard-widgets';
import type { DashboardLayoutItem } from './layout.util';

const CATEGORY_ORDER: readonly WidgetCategory[] = ['revenue', 'appointment', 'customer', 'marketing', 'service', 'care'];

/** Từ khoá không dấu, chữ thường, tách theo ký tự ngoài [a-z0-9], bỏ trùng. */
function searchTokens(query: string): string[] {
  return [...new Set(normalizeSearchText(query).split(/[^a-z0-9]+/).filter(Boolean))];
}

/**
 * Lọc khối theo chuỗi tìm KHÔNG dấu trên tên + mô tả đã dịch ("lich" khớp "Lịch hẹn"); nhiều từ ⇒ AND, không theo thứ
 * tự. CRM chưa có bộ `text-search` của ERP ⇒ dùng `normalizeSearchText` sẵn có (bỏ dấu + đ→d + chữ thường).
 */
export function filterWidgets(defs: readonly DashboardWidgetDef[], query: string, label: (k: string) => string): DashboardWidgetDef[] {
  const tokens = searchTokens(query);
  if (tokens.length === 0) return [...defs];
  return defs.filter((w) => {
    const hay = normalizeSearchText(`${label(w.titleKey)} ${label(w.descKey)}`);
    return tokens.every((t) => hay.includes(t));
  });
}

/** Nhóm khối của Kho khối theo chủ đề (thuần — test được): chỉ khối trong `registry` (đã lọc quyền) khớp từ khoá. */
export function galleryGroups(registry: readonly DashboardWidgetDef[], query: string, label: (k: string) => string) {
  const found = filterWidgets(registry, query, label);
  return CATEGORY_ORDER.map((category) => ({
    category,
    labelKey: WIDGET_CATEGORY_LABEL[category],
    items: found.filter((w) => w.category === category),
  })).filter((g) => g.items.length);
}

/**
 * Kho khối (architecture §5 `WidgetGalleryDrawer`): drawer phải, < md toàn màn hình; tìm không dấu; nhóm theo chủ đề;
 * mỗi mục = hình minh hoạ SVG tĩnh + tên + mô tả + "+"/"Đã thêm". Không tự lưu — phát `add` cho trang quyết định
 * (đang sửa ⇒ vào nháp, đang xem ⇒ lưu luôn).
 */
@Component({
  selector: 'app-dashboard-widget-gallery',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [Drawer, InputTextModule, TranslatePipe],
  template: `
    <!-- appendTo body: trong layout, drawer nằm dưới stacking context ⇒ topbar (z 999) che mất tiêu đề + nút đóng. -->
    <p-drawer [(visible)]="visible" position="right" appendTo="body" [fullScreen]="fullScreen()" [header]="'dashboard.crm.layout.gallery_title' | translate"
              styleClass="w-full md:!w-[28rem]" [ariaCloseLabel]="'common.action.close' | translate" (onHide)="query.set('')">
      <div class="flex flex-col gap-4">
        <input pInputText type="search" class="w-full" [value]="query()" (input)="query.set($any($event.target).value)"
               [placeholder]="'dashboard.crm.layout.search_placeholder' | translate"
               [attr.aria-label]="'dashboard.crm.layout.search_placeholder' | translate" />

        @for (g of groups(); track g.category) {
          <section [attr.aria-labelledby]="'dash-gal-' + g.category">
            <h4 [id]="'dash-gal-' + g.category" class="m-0 mb-2 text-xs font-semibold uppercase tracking-wide text-[var(--p-text-muted-color)]">
              {{ g.labelKey | translate }}
            </h4>
            <ul class="m-0 p-0 list-none flex flex-col gap-2">
              @for (w of g.items; track w.key) {
                <li class="flex items-center gap-3 p-3 rounded-xl border border-[var(--p-content-border-color)] bg-[var(--p-content-background)]">
                  <div class="shrink-0 w-16 h-12 rounded-lg grid place-items-center bg-[var(--p-content-hover-background)] text-[var(--p-primary-color)]" aria-hidden="true">
                    <svg viewBox="0 0 48 32" class="w-12 h-8" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                      @switch (w.thumb) {
                        @case ('kpi') { <rect x="3" y="5" width="18" height="22" rx="3" /><rect x="27" y="5" width="18" height="22" rx="3" /><path d="M7 20h10M31 20h10" /> }
                        @case ('bar') { <path d="M6 28V18M15 28V10M24 28V14M33 28V6M42 28V20" stroke-width="4" /> }
                        @case ('donut') { <circle cx="24" cy="16" r="11" /><path d="M24 5a11 11 0 0 1 10.5 14.5" stroke-width="5" /> }
                        @case ('funnel') { <path d="M6 6h36M11 14h26M16 22h16M21 29h6" stroke-width="4" /> }
                        @case ('map') { <path d="M5 8l12-4 14 4 12-4v20l-12 4-14-4-12 4z" /><path d="M17 4v20M31 8v20" /> }
                        @default { <path d="M6 8h36M6 16h36M6 24h24" /> }
                      }
                    </svg>
                  </div>
                  <div class="min-w-0 flex-1">
                    <div class="text-sm font-medium text-[var(--p-text-color)]">{{ w.titleKey | translate }}</div>
                    <div class="text-xs text-[var(--p-text-muted-color)] line-clamp-3" [attr.title]="w.descKey | translate">{{ w.descKey | translate }}</div>
                  </div>
                  @if (shown().has(w.key)) {
                    <span class="shrink-0 inline-flex items-center gap-1 text-xs text-[var(--p-text-muted-color)]">
                      <i class="pi pi-check" aria-hidden="true"></i>{{ 'dashboard.crm.layout.added' | translate }}
                    </span>
                  } @else {
                    <button type="button" [disabled]="busy()"
                            class="shrink-0 w-9 h-9 rounded-full grid place-items-center text-[var(--p-primary-color)] border border-[var(--p-content-border-color)] hover:bg-[var(--p-content-hover-background)] focus-visible:outline-2 focus-visible:outline-[var(--p-primary-color)] disabled:opacity-50"
                            [attr.aria-label]="('common.action.add' | translate) + ': ' + (w.titleKey | translate)"
                            (click)="add.emit(w.key)">
                      <i class="pi pi-plus" aria-hidden="true"></i>
                    </button>
                  }
                </li>
              }
            </ul>
          </section>
        } @empty {
          <p class="m-0 py-8 text-center text-sm text-[var(--p-text-muted-color)]">{{ 'dashboard.crm.layout.no_results' | translate }}</p>
        }
      </div>
    </p-drawer>
  `,
})
export class DashboardWidgetGallery {
  readonly visible = model(false);
  /** Registry đã lọc quyền (`DashboardLayoutStore.registry`) — khối user không được thấy KHÔNG có trong kho. */
  readonly registry = input.required<readonly DashboardWidgetDef[]>();
  readonly items = input.required<readonly DashboardLayoutItem[]>();
  readonly fullScreen = input(false);
  readonly busy = input(false);
  readonly add = output<string>();

  protected readonly query = signal('');
  protected readonly shown = computed(() => new Set(this.items().filter((x) => x.visible).map((x) => x.key)));

  // `stream` ⇒ tìm theo đúng ngôn ngữ đang chọn, cả khi bản dịch nạp muộn.
  // Khoá của MỌI khối (không chỉ registry đã lọc): danh sách cố định ⇒ 1 stream cả đời component.
  private readonly labels = toSignal(
    inject(TranslateService).stream(CRM_DASHBOARD_WIDGETS.flatMap((w) => [w.titleKey, w.descKey])) as Observable<Record<string, string>>,
    { initialValue: {} as Record<string, string> },
  );

  protected readonly groups = computed(() => {
    const labels = this.labels();
    return galleryGroups(this.registry(), this.query(), (k) => labels[k] ?? k);
  });
}
