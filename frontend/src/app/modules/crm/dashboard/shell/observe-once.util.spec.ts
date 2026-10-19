import { describe, expect, it, vi } from 'vitest';

import { observeOnce } from './observe-once.util';

class FakeIO {
  static last: FakeIO;
  disconnect = vi.fn();
  observe = vi.fn();
  constructor(public cb: (e: { isIntersecting: boolean }[]) => void, public opts: unknown) {
    FakeIO.last = this;
  }
}

describe('observeOnce', () => {
  const IO = FakeIO as unknown as typeof IntersectionObserver;

  it('gọi đúng 1 lần khi vào viewport rồi disconnect', () => {
    const cb = vi.fn();
    observeOnce(document.createElement('div'), cb, undefined, IO);
    FakeIO.last.cb([{ isIntersecting: false }]);
    expect(cb).not.toHaveBeenCalled();
    FakeIO.last.cb([{ isIntersecting: true }]);
    FakeIO.last.cb([{ isIntersecting: true }]);
    expect(cb).toHaveBeenCalledTimes(1);
    expect(FakeIO.last.disconnect).toHaveBeenCalled();
  });
  it('huỷ trước khi hiện ⇒ disconnect, không gọi về sau', () => {
    const cb = vi.fn();
    const stop = observeOnce(document.createElement('div'), cb, undefined, IO);
    stop();
    expect(FakeIO.last.disconnect).toHaveBeenCalledTimes(1);
    FakeIO.last.cb([{ isIntersecting: true }]);
    expect(cb).not.toHaveBeenCalled();
  });
  it('rootMargin mặc định 600px', () => {
    observeOnce(document.createElement('div'), () => {}, undefined, IO);
    expect(FakeIO.last.opts).toEqual({ threshold: 0, rootMargin: '600px 0px' });
  });
  it('môi trường không có IntersectionObserver ⇒ gọi ngay', () => {
    const cb = vi.fn();
    observeOnce(document.createElement('div'), cb, undefined, null);
    expect(cb).toHaveBeenCalledTimes(1);
  });
});
