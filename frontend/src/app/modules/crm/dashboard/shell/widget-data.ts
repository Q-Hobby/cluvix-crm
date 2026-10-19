import { computed, inject, InjectionToken, Signal, signal } from '@angular/core';
import { takeUntilDestroyed, toObservable } from '@angular/core/rxjs-interop';
import { catchError, combineLatest, map, Observable, of, switchMap } from 'rxjs';

import { CrmDashboardDataCache } from '@/core/services/crm/crm-dashboard-data-cache.service';
import type { DashboardQuery } from '@/core/services/crm/crm-dashboard.service';

import { CRM_DASHBOARD_WIDGET_MAP, type WidgetCategory, type WidgetTimeMode } from './dashboard-widgets';
import { DashboardFilterStore } from './dashboard-filter.store';
import type { WidgetStatus } from './widget-frame';

/**
 * Ngữ cảnh shell cấp cho từng khối (story-09/10 provide qua injector của NgComponentOutlet): bật mục "Ẩn khối" và
 * nhận sự kiện ẩn. Khối render ngoài shell (dashboard cũ) ⇒ không có context ⇒ không có mục Ẩn.
 */
export interface DashboardWidgetContext {
  canHide: Signal<boolean>;
  hide(): void;
  /** Màu nhấn theo nhóm khối (`WIDGET_CATEGORY_ACCENT`) — ô KPI tint nền/viền. */
  accent?: string;
  /** Nhóm khối ⇒ icon + nhãn nhóm ở tiêu đề khung. */
  category?: WidgetCategory;
}
export const DASHBOARD_WIDGET_CONTEXT = new InjectionToken<DashboardWidgetContext>('DASHBOARD_WIDGET_CONTEXT');

export interface WidgetData<T> {
  status: Signal<WidgetStatus>;
  value: Signal<T | null>;
  /** Nhãn kỳ cho header (key i18n + tham số — khung tự dịch bằng pipe ⇒ đổi theo ngôn ngữ). */
  period: Signal<PeriodLabel | null>;
  retry(): void;
  refresh(): void;
}

const EMPTY_QUERY: DashboardQuery = {};

/**
 * Khoá cache số liệu khối — PHẢI gồm bộ lọc (khớp mô tả `CrmDashboardDataCache`). CRM: 1 DB = 1 thương hiệu ⇒ không
 * kèm công ty như ERP; đổi tài khoản thì cache bị xoá khi đăng xuất.
 */
export function widgetCacheKey(key: string, q: DashboardQuery): string {
  return `${key}|${q.from ?? 0}|${q.to ?? 0}|${q.branch_id ?? 0}`;
}

export interface PeriodLabel {
  key: string;
  params?: Record<string, string>;
}

/**
 * Dữ liệu 1 khối Dashboard CRM: đọc bộ lọc chung (story-06), lấy số qua `CrmDashboardDataCache` (stale-while-
 * revalidate, khoá `<key>|<from>|<to>|<branch>`), đổi bộ lọc ⇒ `switchMap` huỷ request cũ.
 * PHẢI gọi trong injection context (field initializer của component).
 *
 * `DashboardFilterStore` inject TUỲ CHỌN: dashboard cũ chưa provide store ⇒ dùng kỳ mặc định (query rỗng) — trang
 * vẫn chạy y như trước khi gắn shell (story-09).
 */
export function createWidgetData<T>(
  key: string,
  loader: (q: DashboardQuery) => Observable<T>,
  isEmpty: (v: T) => boolean = () => false,
): WidgetData<T> {
  const store = inject(DashboardFilterStore, { optional: true });
  const cache = inject(CrmDashboardDataCache);
  const timeMode: WidgetTimeMode = CRM_DASHBOARD_WIDGET_MAP.get(key)?.timeMode ?? 'range';

  const status = signal<WidgetStatus>('loading');
  const value = signal<T | null>(null);
  const reload = signal(0);
  /** true ⇒ lượt kế bỏ qua cache (Làm mới). */
  let bypass = false;
  /** Khoá bộ lọc của số đang hiện — đổi khoá ⇒ về `loading` (không để số kỳ CŨ nằm dưới nhãn kỳ MỚI — QA P1). */
  let lastKey: string | null = null;

  const query = computed(() => (store ? store.query() : EMPTY_QUERY));
  combineLatest([toObservable(query), toObservable(reload)])
    .pipe(
      switchMap(([q]) => {
        const k = widgetCacheKey(key, q);
        const skipCache = bypass;
        bypass = false;
        // Cùng khoá (Thử lại/Làm mới) ⇒ giữ số đang hiện; khác khoá ⇒ loading. Cache trúng phát ĐỒNG BỘ ⇒ không nháy.
        if (k !== lastKey || value() === null || status() === 'error') status.set('loading');
        lastKey = k;
        const src$ = skipCache
          ? loader(q).pipe(map((v) => ({ value: v, stale: false })))
          : cache.fetch(k, () => loader(q));
        return src$.pipe(
          map((r) => ({ ok: true as const, value: r.value })),
          catchError(() => of({ ok: false as const, value: null })),
        );
      }),
      takeUntilDestroyed(),
    )
    .subscribe((r) => {
      if (!r.ok) {
        // Interceptor đã toast lỗi (B.5) — chỉ chuyển trạng thái.
        status.set('error');
        return;
      }
      value.set(r.value as T);
      status.set(isEmpty(r.value as T) ? 'empty' : 'ready');
    });

  const period = computed<PeriodLabel | null>(() => {
    if (timeMode === 'snapshot') return { key: 'dashboard.crm.widget.no_period' };
    if (timeMode !== 'day' && timeMode !== 'mixed') return null;
    // Không có store (dashboard cũ) ⇒ kỳ mặc định ⇒ ngày tham chiếu = hôm nay.
    const l = store ? store.refDayLabel() : ({ kind: 'today' } as const);
    return l.kind === 'today' ? { key: 'common.label.today' } : { key: 'dashboard.crm.filter.ref_day', params: { date: l.date } };
  });

  return {
    status,
    value,
    period,
    retry: () => reload.update((n) => n + 1),
    refresh: () => {
      bypass = true;
      reload.update((n) => n + 1);
    },
  };
}

/** Ngữ cảnh shell (nếu có) cho DashboardWidgetFrame: canHide + hide. */
export function injectWidgetContext(): DashboardWidgetContext | null {
  return inject(DASHBOARD_WIDGET_CONTEXT, { optional: true });
}
