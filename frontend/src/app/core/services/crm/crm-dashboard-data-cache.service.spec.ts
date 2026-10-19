import { firstValueFrom, lastValueFrom, of, throwError, toArray } from 'rxjs';
import { describe, expect, it, vi } from 'vitest';

import { CRM_DASHBOARD_DATA_TTL_MS, CrmDashboardDataCache } from './crm-dashboard-data-cache.service';

describe('CrmDashboardDataCache', () => {
  it('lần đầu: chỉ gọi API, stale=false', async () => {
    const c = new CrmDashboardDataCache();
    const loader = vi.fn(() => of(1));
    const out = await lastValueFrom(c.fetch('k', loader).pipe(toArray()));
    expect(out).toEqual([{ value: 1, stale: false }]);
    expect(loader).toHaveBeenCalledTimes(1);
  });
  it('trong TTL: phát số cũ NGAY rồi làm mới nền', async () => {
    const c = new CrmDashboardDataCache();
    let t = 1000;
    await lastValueFrom(c.fetch('k', () => of(1), () => t));
    t += 1000;
    const loader = vi.fn(() => of(2));
    const out = await lastValueFrom(c.fetch('k', loader, () => t).pipe(toArray()));
    expect(out).toEqual([{ value: 1, stale: true }, { value: 2, stale: false }]);
    expect(loader).toHaveBeenCalledTimes(1);
  });
  it('hết TTL: chỉ gọi mới, không phát số cũ', async () => {
    const c = new CrmDashboardDataCache();
    let t = 0;
    await lastValueFrom(c.fetch('k', () => of(1), () => t));
    t += CRM_DASHBOARD_DATA_TTL_MS;
    const out = await lastValueFrom(c.fetch('k', () => of(3), () => t).pipe(toArray()));
    expect(out).toEqual([{ value: 3, stale: false }]);
  });
  it('khác khoá (khác công ty/bộ lọc) ⇒ không dùng số của khoá khác', async () => {
    const c = new CrmDashboardDataCache();
    await lastValueFrom(c.fetch('w|1|0|0|0', () => of('cty1')));
    const out = await lastValueFrom(c.fetch('w|2|0|0|0', () => of('cty2')).pipe(toArray()));
    expect(out).toEqual([{ value: 'cty2', stale: false }]);
  });
  it('lỗi khi đã có số cũ ⇒ giữ số cũ, không ném lỗi; chưa có số ⇒ ném lỗi', async () => {
    const c = new CrmDashboardDataCache();
    await lastValueFrom(c.fetch('k', () => of(1)));
    const out = await lastValueFrom(c.fetch('k', () => throwError(() => new Error('500'))).pipe(toArray()));
    expect(out).toEqual([{ value: 1, stale: true }]);
    await expect(firstValueFrom(c.fetch('k2', () => throwError(() => new Error('500'))))).rejects.toThrow('500');
  });
  it('clear() xoá sạch', async () => {
    const c = new CrmDashboardDataCache();
    await lastValueFrom(c.fetch('k', () => of(1)));
    c.clear();
    const out = await lastValueFrom(c.fetch('k', () => of(9)).pipe(toArray()));
    expect(out).toEqual([{ value: 9, stale: false }]);
  });
  it('không đụng storage trình duyệt', async () => {
    const set = vi.spyOn(Storage.prototype, 'setItem');
    const c = new CrmDashboardDataCache();
    await lastValueFrom(c.fetch('k', () => of({ doanhThu: 123 })));
    expect(set).not.toHaveBeenCalled();
    set.mockRestore();
  });
});
