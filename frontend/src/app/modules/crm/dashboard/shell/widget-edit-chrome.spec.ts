import { describe, expect, it } from 'vitest';

import { moveCommands } from './widget-edit-chrome';

const kinds = (z: 'top' | 'main' | 'side', i: number, n: number) =>
  moveCommands(z, i, n).map((m) => (m.cmd.kind === 'zone' ? `zone:${m.cmd.zone}` : m.cmd.kind));

describe('moveCommands (menu di chuyển — story-10 AC3)', () => {
  it('khối đầu vùng: không có Lên; khối cuối: không có Xuống; khối duy nhất: không có cả hai', () => {
    expect(kinds('main', 0, 3)).toEqual(['down', 'zone:side', 'zone:top']);
    expect(kinds('main', 2, 3)).toEqual(['up', 'zone:side', 'zone:top']);
    expect(kinds('main', 1, 3)).toEqual(['up', 'down', 'zone:side', 'zone:top']);
    expect(kinds('side', 0, 1)).toEqual(['zone:main', 'zone:top']);
  });
  it('không bao giờ có lệnh chuyển sang CHÍNH vùng đang đứng', () => {
    expect(kinds('top', 0, 1)).toEqual(['zone:main', 'zone:side']);
  });
});
