import { Location } from '@angular/common';
import { computed, DestroyRef, inject, Injectable, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { ActivatedRoute, Router } from '@angular/router';

import { branchListId } from '@/core/services/auth/auth.selectors';
import { TimezoneService } from '@/core/services/timezone.service';

import {
  DashboardFilterState, DashboardPreset, DashboardQuery, effectiveRange, nextMidnight, parseUrlParams, refDayStart,
  sameQuery, toApiQuery, toUrlParams,
} from './dashboard-filter.util';

/** Nhãn ngày tham chiếu: hôm nay, hoặc "dd/MM" của ngày tham chiếu. */
export type RefDayLabel = { kind: 'today' } | { kind: 'date'; date: string };

/**
 * Bộ lọc chung Dashboard CRM (PRD FR10–FR15). PROVIDE ở component Dashboard (không root) — mỗi lần vào trang
 * đọc lại từ URL.
 *
 * URL: ghi bằng `Location.go` (push lịch sử, KHÔNG chạy điều hướng router ⇒ không vẽ lại trang — B.19).
 * Back/Forward: router KHÔNG phát `queryParamMap` (nó coi URL sau `Location.go` là "cùng URL hiện tại" ⇒
 * `NavigationSkipped`) ⇒ nghe trực tiếp `Location.subscribe` (popstate) và đọc lại (QA story-06 P1).
 *
 * "Bây giờ": computed đọc `nowTick` — làm mới lúc nửa đêm theo giờ thương hiệu và khi tab hiện lại, để trang mở qua
 * đêm không còn gửi kỳ "hôm nay" của hôm qua (QA P4).
 */
@Injectable()
export class DashboardFilterStore {
  private readonly tz = inject(TimezoneService);
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly location = inject(Location);

  /** Giờ treo tường thương hiệu ⇒ Unix đầu ngày (xem `dashboard-filter.util.ts`). */
  readonly wallToUnix = (d: Date) => this.tz.dateHmsToUnix(d, 0, 0, 0);
  readonly unixToWall = (u: number) => this.tz.fromUnix(u) as Date;

  private readonly _state = signal<DashboardFilterState>(this.readUrl());
  /** Đổi giá trị ⇒ mọi computed theo "bây giờ" tính lại. */
  private readonly nowTick = signal(0);
  readonly state = this._state.asReadonly();

  /** [from, to) hiệu lực (kể cả preset mặc định) — dùng cho nhãn & tính ngày tham chiếu. */
  readonly range = computed(
    () => {
      this.nowTick();
      return effectiveRange(this._state(), this.tz.now(), this.wallToUnix);
    },
    // Cùng kỳ ⇒ cùng tham chiếu: nhịp đồng hồ/visibility KHÔNG được làm khối gọi lại API (QA P11).
    { equal: (a, b) => a.from === b.from && a.to === b.to },
  );
  /** Tham số gửi BE — mặc định là `{}` (không gửi gì ⇒ BE đi đường cũ). */
  readonly query = computed<DashboardQuery>(
    () => {
      this.nowTick();
      return toApiQuery(this._state(), this.tz.now(), this.wallToUnix);
    },
    { equal: sameQuery },
  );
  /** Khoá ổn định của bộ lọc — nối vào khoá cache số liệu khối. */
  readonly cacheKey = computed(() => {
    const q = this.query();
    return `${q.from ?? 0}|${q.to ?? 0}|${q.branch_id ?? 0}`;
  });
  readonly refDay = computed(() => {
    this.nowTick();
    return refDayStart(this.range().to, Math.floor(Date.now() / 1000), this.unixToWall, this.wallToUnix);
  });
  readonly refDayLabel = computed<RefDayLabel>(() => {
    this.nowTick();
    const ref = this.refDay();
    if (ref === this.tz.todayStartUnix()) return { kind: 'today' };
    const w = this.unixToWall(ref);
    return { kind: 'date', date: `${String(w.getDate()).padStart(2, '0')}/${String(w.getMonth() + 1).padStart(2, '0')}` };
  });

  constructor() {
    const destroyRef = inject(DestroyRef);
    // Điều hướng router thật (vd từ menu vào lại trang với query khác).
    this.route.queryParamMap.pipe(takeUntilDestroyed(destroyRef)).subscribe(() => this.syncFromUrl());
    // Back/Forward sau `Location.go`.
    const popSub = this.location.subscribe(() => this.syncFromUrl());
    destroyRef.onDestroy(() => popSub.unsubscribe());
    this.startClock(destroyRef);
  }

  private syncFromUrl(): void {
    const next = this.readUrl();
    if (!sameState(next, this._state())) this._state.set(next);
  }

  /** Hẹn giờ tới nửa đêm kế tiếp (giờ thương hiệu) + nghe tab hiện lại ⇒ tăng `nowTick`. */
  private startClock(destroyRef: DestroyRef): void {
    let timer: ReturnType<typeof setTimeout> | undefined;
    const schedule = () => {
      // Nửa đêm kế tiếp theo NGÀY treo tường (không `todayStart + 86400`: ngày DST 25h ⇒ bắn mỗi giây suốt 1 giờ — QA P11).
      const ms = Math.max(1000, nextMidnight(this.tz.now(), this.wallToUnix) * 1000 - Date.now() + 1000);
      timer = setTimeout(() => {
        this.nowTick.update((v) => v + 1);
        schedule();
      }, ms);
    };
    schedule();
    const onVisible = () => {
      if (typeof document !== 'undefined' && document.visibilityState === 'visible') this.nowTick.update((v) => v + 1);
    };
    if (typeof document !== 'undefined') document.addEventListener('visibilitychange', onVisible);
    destroyRef.onDestroy(() => {
      clearTimeout(timer);
      if (typeof document !== 'undefined') document.removeEventListener('visibilitychange', onVisible);
    });
  }

  setPreset(preset: Exclude<DashboardPreset, 'custom'>): void {
    this.update({ ...this._state(), preset, from: null, to: null });
  }

  setCustom(from: number, to: number): void {
    this.update({ ...this._state(), preset: 'custom', from, to });
  }

  setBranch(branchId: number | null): void {
    this.update({ ...this._state(), branchId: branchId || null });
  }

  private update(next: DashboardFilterState): void {
    if (sameState(next, this._state())) return;
    this._state.set(next);
    const tree = this.router.createUrlTree([], {
      relativeTo: this.route,
      queryParams: toUrlParams(next),
      queryParamsHandling: 'merge',
    });
    this.location.go(this.router.serializeUrl(tree));
  }

  private readUrl(): DashboardFilterState {
    // Đọc URL THẬT của trình duyệt: sau `Location.go` snapshot của route không đổi theo.
    const tree = this.router.parseUrl(this.location.path(false) || this.router.url);
    const params = tree.queryParamMap;
    return parseUrlParams((k) => params.get(k), branchListId() ?? []);
  }
}

function sameState(a: DashboardFilterState, b: DashboardFilterState): boolean {
  return a.preset === b.preset && a.from === b.from && a.to === b.to && a.branchId === b.branchId;
}
