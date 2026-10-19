import { Location } from '@angular/common';
import { provideLocationMocks } from '@angular/common/testing';
import { inject } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, provideRouter, Router } from '@angular/router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { DashboardFilterStore } from './dashboard-filter.store';

const flush = async () => {
  for (let i = 0; i < 5; i++) await new Promise((r) => setTimeout(r, 0));
};

async function setup(url: string) {
  TestBed.configureTestingModule({
    providers: [
      provideRouter([{ path: 'd', children: [] }]),
      provideLocationMocks(),
      { provide: ActivatedRoute, useFactory: () => inject(Router).routerState.root },
      DashboardFilterStore,
    ],
  });
  const router = TestBed.inject(Router);
  const loc = TestBed.inject(Location);
  (router as unknown as { setUpLocationChangeListener(): void }).setUpLocationChangeListener();
  await router.navigateByUrl(url);
  return { store: TestBed.inject(DashboardFilterStore), loc };
}

describe('DashboardFilterStore', () => {
  it('mặc định: query rỗng (không gửi gì lên BE)', async () => {
    const { store } = await setup('/d');
    expect(store.state().preset).toBe('this_month');
    expect(store.query()).toEqual({});
  });

  it('đổi preset ⇒ query có from/to, URL có dr, giữ tham số khác của trang', async () => {
    const { store, loc } = await setup('/d?keep=1');
    store.setPreset('last_month');
    expect(store.query().from).toBeTypeOf('number');
    expect(store.query().to! > store.query().from!).toBe(true);
    expect(loc.path()).toContain('dr=last_month');
    expect(loc.path()).toContain('keep=1');
  });

  it('Back/Forward đổi trạng thái theo URL (QA P1)', async () => {
    const { store, loc } = await setup('/d?keep=1');
    store.setPreset('today');
    store.setBranch(5);
    expect(store.state()).toMatchObject({ preset: 'today', branchId: 5 });
    loc.back();
    await flush();
    expect(store.state()).toMatchObject({ preset: 'today', branchId: null });
    loc.back();
    await flush();
    expect(store.state().preset).toBe('this_month');
    expect(store.query()).toEqual({});
    loc.forward();
    await flush();
    expect(store.state().preset).toBe('today');
  });

  it('Back về ĐÚNG URL lúc vào trang — router coi là "cùng URL" (NavigationSkipped), store phải tự nghe popstate', async () => {
    const { store, loc } = await setup('/d?keep=1');
    store.setPreset('today');
    loc.back();
    await flush();
    expect(loc.path()).toBe('/d?keep=1');
    expect(store.state().preset).toBe('this_month');
    expect(store.query()).toEqual({});
  });

  it('đọc URL lúc vào trang (reload / link chia sẻ)', async () => {
    const { store } = await setup('/d?dr=this_year');
    expect(store.state().preset).toBe('this_year');
  });

  it('refDay của kỳ mặc định là hôm nay', async () => {
    const { store } = await setup('/d');
    expect(store.refDayLabel()).toEqual({ kind: 'today' });
  });

  describe('theo đồng hồ (Date giả)', () => {
    afterEach(() => vi.useRealTimers());

    it('kỳ tháng trước ⇒ nhãn ngày tham chiếu dd/MM = ngày cuối tháng trước', async () => {
      vi.useFakeTimers({ toFake: ['Date'], now: new Date('2026-10-08T08:30:00Z') }); // 15:30 giờ VN
      const { store } = await setup('/d?dr=last_month');
      expect(store.refDayLabel()).toEqual({ kind: 'date', date: '30/09' });
    });

    it('qua nửa đêm (giờ công ty) với preset "Hôm nay" ⇒ query tự sang ngày mới (QA P4/P12)', async () => {
      vi.useFakeTimers({ toFake: ['Date', 'setTimeout', 'clearTimeout'], now: new Date('2026-10-08T16:58:00Z') }); // 23:58 giờ VN
      const { store } = await setup('/d?dr=today');
      const before = store.query();
      expect(before.to! - before.from!).toBe(86400);
      vi.advanceTimersByTime(4 * 60 * 1000); // qua 00:00 giờ VN
      const after = store.query();
      expect(after.from).toBe(before.from! + 86400);
      expect(store.refDayLabel()).toEqual({ kind: 'today' });
    });

    it('nhịp đồng hồ không đổi kỳ ⇒ query giữ NGUYÊN tham chiếu (không làm khối gọi lại API — QA P11)', async () => {
      vi.useFakeTimers({ toFake: ['Date'], now: new Date('2026-10-08T08:30:00Z') });
      const { store } = await setup('/d?dr=last_month');
      const q1 = store.query();
      document.dispatchEvent(new Event('visibilitychange'));
      expect(store.query()).toBe(q1);
    });
  });
});
