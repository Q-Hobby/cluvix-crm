import { describe, expect, it } from 'vitest';

import { serviceStatsSeries, sourceNestedPie } from './chart-transforms';

describe('chart-transforms', () => {
  it('tên dài GIỮ ĐẦY ĐỦ trong dữ liệu (tooltip cần) — rút gọn chỉ ở nhãn của directive (/check-ui 2026-10-09)', () => {
    const long = 'Chiến dịch tri ân khách hàng thân thiết quý IV năm 2026';
    const { inner, outer } = sourceNestedPie([{ name: long, value: 5, group: long }], { otherSources: 'OS', otherGroups: 'OG' });
    expect(outer[0].name).toBe(long);
    expect(inner[0].name).toBe(long);
    expect(serviceStatsSeries([{ name: long, quantity: 1, revenue: '1' }]).categories).toEqual([long]);
  });

  it('sourceNestedPie: top 10 nguồn + "khác", top 5 nhóm + "khác", sắp giảm dần', () => {
    const rows = Array.from({ length: 12 }, (_, i) => ({ name: `Nguồn ${i}`, value: i + 1, group: `G${i % 7}` }));
    const { inner, outer } = sourceNestedPie(rows, { otherSources: 'OS', otherGroups: 'OG' });
    expect(outer).toHaveLength(11);
    expect(outer[0]).toEqual({ name: 'Nguồn 11', value: 12 });
    expect(outer[10]).toEqual({ name: 'OS', value: 1 + 2 });
    expect(inner).toHaveLength(6);
    expect(inner[5].name).toBe('OG');
    const total = rows.reduce((s, r) => s + r.value, 0);
    expect(inner.reduce((s, r) => s + r.value, 0)).toBe(total);
    expect(outer.reduce((s, r) => s + r.value, 0)).toBe(total);
  });

  it('sourceNestedPie: ít mục ⇒ không có "khác"', () => {
    const { inner, outer } = sourceNestedPie([{ name: 'A', value: 1, group: 'G' }], { otherSources: 'OS', otherGroups: 'OG' });
    expect(outer).toEqual([{ name: 'A', value: 1 }]);
    expect(inner).toEqual([{ name: 'G', value: 1 }]);
  });

  it('serviceStatsSeries: doanh thu chuỗi decimal BE ⇒ số', () => {
    expect(serviceStatsSeries([{ name: 'X', quantity: 2, revenue: '6800000000' }])).toEqual({
      categories: ['X'], quantities: [2], revenues: [6800000000],
    });
  });
});
