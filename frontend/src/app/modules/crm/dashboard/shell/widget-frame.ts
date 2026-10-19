import { ChangeDetectionStrategy, Component, computed, inject, input, output } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { TranslatePipe, TranslateService } from '@ngx-translate/core';
import { MenuItem } from 'primeng/api';
import { MenuModule } from 'primeng/menu';
import { Skeleton } from 'primeng/skeleton';
import { TooltipModule } from 'primeng/tooltip';

import { ink, tint } from '@/share/utils/dashboard-tint.util';

import { WIDGET_CATEGORY_ACCENT, WIDGET_CATEGORY_ICON, type WidgetCategory, type WidgetSkeleton } from './dashboard-widgets';
import { injectWidgetContext, type PeriodLabel } from './widget-data';

export type WidgetStatus = 'loading' | 'empty' | 'error' | 'ready';

/**
 * Khung DÙNG CHUNG cho mọi khối Dashboard CRM (epic crm-dashboard-customizable, PRD FR21):
 * tiêu đề + tooltip định nghĩa + nhãn kỳ + menu "…" (Làm mới · Xem báo cáo · Ẩn) và 4 trạng thái
 * loading/empty/error/ready. Nội dung khối chiếu qua `<ng-content>` khi `ready`.
 * Màu theo token theme (B.18) — KHÔNG hardcode `bg-white`/`text-gray-*`.
 */
@Component({
  selector: 'app-dashboard-widget-frame',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [TranslatePipe, Skeleton, TooltipModule, MenuModule],
  template: `
    <section
      class="h-full flex flex-col gap-3 p-4 rounded-xl border border-[var(--p-content-border-color)] bg-[var(--p-content-background)] shadow-sm"
      [attr.aria-busy]="status() === 'loading'">
      <header class="flex items-start gap-2.5 min-w-0">
        @if (cat(); as c) {
          <!-- Ô icon màu theo nhóm khối (mẫu đã duyệt 2026-10-09): nhận diện nhóm khi bật nhiều khối trên trang. -->
          <span class="shrink-0 grid place-items-center w-10 h-10 rounded-lg" aria-hidden="true"
                [style.background]="tint(accent(c), 14)" [style.color]="ink(accent(c))">
            <i [class]="icon(c) + ' text-base'"></i>
          </span>
        }
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-1.5 min-w-0">
            <h3 class="m-0 text-lg font-bold text-[var(--p-text-color)] truncate" [attr.title]="titleKey() | translate">{{ titleKey() | translate }}</h3>
            @if (infoKey()) {
              <button type="button"
                      class="shrink-0 inline-flex items-center justify-center w-8 h-8 -my-1 rounded-md text-[var(--p-text-muted-color)] hover:bg-[var(--p-content-hover-background)] focus-visible:outline-2 focus-visible:outline-[var(--p-primary-color)]"
                      [pTooltip]="infoKey()! | translate" tooltipPosition="top"
                      [attr.aria-label]="'dashboard.crm.widget.definition' | translate">
                <i class="pi pi-info-circle text-xs" aria-hidden="true"></i>
              </button>
            }
          </div>
          @if (period(); as p) {
            <p class="m-0 mt-0.5 text-xs text-[var(--p-text-muted-color)] truncate" [attr.title]="p.key | translate: p.params">{{ p.key | translate: p.params }}</p>
          }
        </div>
        @if (menuItems().length) {
          <button type="button"
                  class="shrink-0 inline-flex items-center justify-center w-8 h-8 rounded-lg text-[var(--p-text-muted-color)] hover:bg-[var(--p-content-hover-background)] focus-visible:outline-2 focus-visible:outline-[var(--p-primary-color)]"
                  aria-haspopup="menu"
                  [attr.aria-label]="'common.action.more_actions' | translate"
                  (click)="frameMenu.toggle($event)">
            <i class="pi pi-ellipsis-h" aria-hidden="true"></i>
          </button>
          <p-menu #frameMenu [model]="menuItems()" [popup]="true" appendTo="body" />
        }
      </header>

      <!-- container-type: lưới ô KPI trong khối co theo bề rộng KHỐI (cỡ ½, cột phụ), không theo màn hình (smoke F3). -->
      <div class="flex-1 min-h-0 [container-type:inline-size]">
        @switch (status()) {
          @case ('loading') {
            @switch (skeleton()) {
              @case ('kpi') {
                <div class="grid grid-cols-2 gap-3">
                  @for (i of [1, 2, 3, 4]; track i) {
                    <div class="flex flex-col gap-2 p-3 rounded-lg border border-[var(--p-content-border-color)]">
                      <p-skeleton width="55%" height="0.7rem" />
                      <p-skeleton width="75%" height="1.4rem" />
                    </div>
                  }
                </div>
              }
              @case ('list') {
                <div class="flex flex-col gap-2">
                  @for (i of [1, 2, 3, 4, 5]; track i) {
                    <p-skeleton width="100%" height="1.6rem" />
                  }
                </div>
              }
              @default {
                <p-skeleton width="100%" [height]="(bodyHeight() ?? 220) + 'px'" borderRadius="0.75rem" />
              }
            }
          }
          @case ('error') {
            <div class="h-full min-h-32 flex flex-col items-center justify-center gap-2 text-center" role="alert">
              <i class="pi pi-exclamation-triangle text-xl text-[var(--p-text-muted-color)]" aria-hidden="true"></i>
              <span class="text-sm text-[var(--p-text-muted-color)]">{{ 'common.message.load_error' | translate }}</span>
              <button type="button"
                      class="px-3 py-1.5 rounded-lg text-sm font-medium text-[var(--p-primary-color)] hover:bg-[var(--p-content-hover-background)] focus-visible:outline-2 focus-visible:outline-[var(--p-primary-color)]"
                      (click)="retry.emit()">
                <i class="pi pi-refresh mr-1" aria-hidden="true"></i>{{ 'common.action.retry' | translate }}
              </button>
            </div>
          }
          @case ('empty') {
            <div class="h-full min-h-32 flex flex-col items-center justify-center gap-2 text-center">
              <i class="pi pi-inbox text-xl text-[var(--p-text-muted-color)]" aria-hidden="true"></i>
              <span class="text-sm text-[var(--p-text-muted-color)]">{{ emptyKey() | translate }}</span>
            </div>
          }
          @default {
            <ng-content />
          }
        }
      </div>
    </section>
  `,
})
export class DashboardWidgetFrame {
  readonly titleKey = input.required<string>();
  /** i18n key mô tả định nghĩa số liệu (tooltip ⓘ). */
  readonly infoKey = input<string | null>(null);
  /** Nhãn kỳ (key i18n + tham số): "Hôm nay", "Ngày 30/09", "Không áp khoảng ngày"… */
  readonly period = input<PeriodLabel | null>(null);
  readonly status = input<WidgetStatus>('loading');
  /** Nhóm khối (icon + màu). Host truyền lúc chờ tải; khối thật lấy từ context shell. Ngoài shell ⇒ không icon. */
  readonly category = input<WidgetCategory | null>(null);
  private readonly ctxCategory = injectWidgetContext()?.category ?? null;
  protected readonly cat = computed(() => this.category() ?? this.ctxCategory);
  protected readonly accent = (c: WidgetCategory) => WIDGET_CATEGORY_ACCENT[c];
  protected readonly icon = (c: WidgetCategory) => WIDGET_CATEGORY_ICON[c];
  protected readonly tint = tint;
  protected readonly ink = ink;
  readonly skeleton = input<WidgetSkeleton>('chart');
  /** Chiều cao vùng biểu đồ (px, từ registry `chartHeight`) ⇒ khung xương cao đúng bằng biểu đồ thật (không nhảy khi dữ liệu về). */
  readonly bodyHeight = input<number | null>(null);
  /** Câu hiện ở trạng thái rỗng (khối có lý do riêng, vd CLV chưa tính). */
  readonly emptyKey = input('dashboard.crm.widget.empty');
  /** Hiện mục "Ẩn khối" trong menu (shell bật khi khối nằm trong bố cục tuỳ biến). */
  readonly canHide = input(false);
  /** Route báo cáo chi tiết; null ⇒ không hiện mục. */
  readonly reportRoute = input<string | null>(null);

  readonly refresh = output<void>();
  readonly retry = output<void>();
  readonly hide = output<void>();

  // Nhãn MenuItem là chuỗi (không qua pipe) ⇒ `stream` để đổi theo ngôn ngữ VÀ khi bản dịch nạp muộn (QA P3).
  private readonly labels = toSignal(
    inject(TranslateService).stream(['common.action.refresh', 'dashboard.crm.widget.view_report', 'common.label.hide']),
    { initialValue: {} as Record<string, string> },
  );

  protected readonly menuItems = computed<MenuItem[]>(() => {
    const items: MenuItem[] = [
      { label: this.labels()['common.action.refresh'], icon: 'pi pi-refresh', command: () => this.refresh.emit() },
    ];
    const route = this.reportRoute();
    if (route) {
      items.push({ label: this.labels()['dashboard.crm.widget.view_report'], icon: 'pi pi-external-link', routerLink: route });
    }
    if (this.canHide()) {
      items.push({ label: this.labels()['common.label.hide'], icon: 'pi pi-eye-slash', command: () => this.hide.emit() });
    }
    return items;
  });
}
