import { describe, expect, it } from 'vitest';

import { createChunkLoader } from './chunk-loader.util';

class W {}

describe('createChunkLoader', () => {
  it('tải thành công ⇒ có type, không failed; gọi lại không tải lần 2', async () => {
    let calls = 0;
    const l = createChunkLoader(() => (calls++, Promise.resolve(W)));
    await l.load();
    await l.load();
    expect(l.type()).toBe(W);
    expect(l.failed()).toBe(false);
    expect(calls).toBe(1);
  });

  it('tải lỗi ⇒ failed (không treo skeleton); Thử lại thành công ⇒ hết failed (QA P1)', async () => {
    let fail = true;
    const l = createChunkLoader(() => (fail ? Promise.reject(new Error('ChunkLoadError')) : Promise.resolve(W)));
    await l.load();
    expect(l.failed()).toBe(true);
    expect(l.type()).toBeNull();
    fail = false;
    const p = l.load();
    expect(l.failed()).toBe(false); // đang thử lại ⇒ quay về skeleton
    await p;
    expect(l.type()).toBe(W);
  });

  it('đang tải ⇒ gọi trùng không bắn request thứ 2', async () => {
    let calls = 0;
    let resolve!: (t: typeof W) => void;
    const l = createChunkLoader(() => (calls++, new Promise((r) => (resolve = r))));
    void l.load();
    void l.load();
    resolve(W);
    await Promise.resolve();
    expect(calls).toBe(1);
  });
});
