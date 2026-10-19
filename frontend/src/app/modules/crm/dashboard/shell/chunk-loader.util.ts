import { signal, Type } from '@angular/core';

/**
 * Trạng thái tải CHUNK JS của 1 khối (story-09 QA P1). Tách khỏi component để test được không cần DOM.
 *
 * `appLazyVisible` chỉ bắn 1 lần ⇒ tải lỗi (mạng chập chờn, deploy bản mới khi tab đang mở ⇒ chunk cũ 404) mà
 * chỉ reset cờ thì skeleton treo mãi: phải lộ `failed` để UI hiện lỗi + nút gọi lại `load()`.
 */
export function createChunkLoader(loadComponent: () => Promise<Type<unknown>>) {
  const type = signal<Type<unknown> | null>(null);
  const failed = signal(false);
  let loading = false;

  return {
    type: type.asReadonly(),
    failed: failed.asReadonly(),
    load(): Promise<void> {
      if (loading || type()) return Promise.resolve();
      loading = true;
      failed.set(false);
      return loadComponent().then(
        (t) => {
          type.set(t);
        },
        () => {
          loading = false;
          failed.set(true);
        },
      );
    },
  };
}
