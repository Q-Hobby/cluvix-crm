import { describe, expect, it } from 'vitest';

import { fullNumber, percentNumber, trendTone } from './kpi.util';

describe('fullNumber', () => {
  it('số đầy đủ theo locale, không viết gọn', () => {
    expect(fullNumber(1_234_567_890, 'vi-VN')).toBe('1.234.567.890');
    expect(fullNumber('320000000', 'vi-VN')).toBe('320.000.000');
  });
  it('rỗng / không phải số ⇒ gạch ngang', () => {
    for (const v of [null, undefined, 'abc', NaN]) expect(fullNumber(v as any, 'vi-VN')).toBe('—');
  });
});

describe('trendTone', () => {
  it('tăng tốt / giảm xấu mặc định', () => {
    expect(trendTone(2)).toBe('good');
    expect(trendTone(1)).toBe('bad');
    expect(trendTone(0)).toBe('neutral');
    expect(trendTone(null)).toBe('neutral');
  });
  it('đảo chiều khi tăng là xấu (hoàn tiền)', () => {
    expect(trendTone(2, false)).toBe('bad');
    expect(trendTone(1, false)).toBe('good');
  });
});

describe('percentNumber', () => {
  it('chuỗi decimal BE ⇒ định dạng theo locale', () => {
    expect(percentNumber('12.5', 'vi-VN')).toBe('12,5');
    expect(percentNumber('12.5', 'en-US')).toBe('12.5');
    expect(percentNumber(2100, 'vi-VN')).toBe('2.100');
    expect(percentNumber(null, 'vi-VN')).toBe('—');
  });
});
