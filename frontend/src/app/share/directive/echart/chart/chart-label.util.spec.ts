import { describe, expect, it } from 'vitest';

import { truncateLabel } from './chart-label.util';

describe('truncateLabel', () => {
  it('giữ tên ngắn, cắt tên dài + "…", null ⇒ rỗng, max ≤ 0 ⇒ giữ nguyên', () => {
    expect(truncateLabel('abc', 5)).toBe('abc');
    expect(truncateLabel('abcdefgh', 5)).toBe('abcde…');
    expect(truncateLabel(null, 5)).toBe('');
    expect(truncateLabel('abcdefgh', 0)).toBe('abcdefgh');
  });
});
