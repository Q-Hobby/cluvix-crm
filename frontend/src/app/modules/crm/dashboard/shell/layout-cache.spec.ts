import { describe, expect, it } from 'vitest';

import { CRM_DASHBOARD_WIDGETS } from './dashboard-widgets';
import { type CachedLayout, type LayoutStorage, ownerKey, readCachedLayout, writeCachedLayout } from './layout-cache';
import { systemDefaultLayout } from './layout.util';

function mem(initial?: unknown): LayoutStorage & { data: unknown } {
  const s = { data: initial as unknown, load: () => s.data, save: (v: unknown) => { s.data = JSON.parse(JSON.stringify(v)); } };
  return s;
}
const userLayout: CachedLayout = { config: systemDefaultLayout(CRM_DASHBOARD_WIDGETS), source: 'user', updatedAt: 1 };

describe('layout-cache', () => {
  it('tách theo user (CRM: 1 thương hiệu/DB, không kèm công ty); ghi bản này không xoá bản khác', () => {
    const st = mem();
    writeCachedLayout(st, ownerKey(1), userLayout);
    writeCachedLayout(st, ownerKey(2), { config: null, source: 'system', updatedAt: 0 });
    expect(readCachedLayout(st, ownerKey(1))).toEqual(userLayout);
    expect(readCachedLayout(st, ownerKey(2))?.source).toBe('system');
    expect(readCachedLayout(st, ownerKey(3))).toBeNull();
  });
  it('nguồn `brand` hợp lệ; `company` (khoá ERP cũ) ⇒ coi như không có', () => {
    const st = mem({ '1': { ...userLayout, source: 'brand' }, '2': { ...userLayout, source: 'company' } });
    expect(readCachedLayout(st, '1')?.source).toBe('brand');
    expect(readCachedLayout(st, '2')).toBeNull();
  });
  it('rác / JSON hỏng / sai version ⇒ coi như không có bản tạm (AC4)', () => {
    for (const junk of ['x', 42, [], { '1': { source: 'user', config: { version: 9, items: [] } } }, { '1': 'rác' }]) {
      expect(readCachedLayout(mem(junk), '1')).toBeNull();
    }
  });
  it('storage ném lỗi khi đọc/ghi ⇒ không văng (AC4)', () => {
    const boom: LayoutStorage = { load: () => { throw new Error('SecurityError'); }, save: () => { throw new Error('QuotaExceeded'); } };
    expect(readCachedLayout(boom, '1')).toBeNull();
    expect(() => writeCachedLayout(boom, '1', userLayout)).not.toThrow();
  });
  it('chưa có uid ⇒ không đọc/ghi', () => {
    const st = mem();
    expect(ownerKey(null)).toBeNull();
    writeCachedLayout(st, ownerKey(0), userLayout);
    expect(st.data).toBeUndefined();
  });
  it('bản tạm CHỈ có bố cục — không có trường số liệu (AC6)', () => {
    const st = mem();
    writeCachedLayout(st, '1', userLayout);
    const json = JSON.stringify(st.data);
    expect(json).not.toMatch(/doanh|thucThu|revenue|amount|value"/i);
  });
});
