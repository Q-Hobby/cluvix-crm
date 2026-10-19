/**
 * Theo dõi 1 phần tử tới khi nó vào gần viewport (rootMargin) thì gọi `onVisible` ĐÚNG 1 lần rồi ngừng.
 * Trả hàm huỷ — caller gọi khi component bị huỷ trước khi phần tử kịp hiện (sửa rò observer của bản cũ
 * `dashboard.ts::setupLazyLoading`, vốn không bao giờ disconnect nếu user không cuộn tới).
 */
export const DASHBOARD_LAZY_ROOT_MARGIN = '600px 0px';

export function observeOnce(
  el: Element,
  onVisible: () => void,
  rootMargin = DASHBOARD_LAZY_ROOT_MARGIN,
  IO: typeof IntersectionObserver | null = globalThis.IntersectionObserver ?? null,
): () => void {
  if (!IO) {
    onVisible();
    return () => {};
  }
  let done = false;
  const io = new IO(
    (entries) => {
      if (done || !entries.some((e) => e.isIntersecting)) return;
      done = true;
      io.disconnect();
      onVisible();
    },
    { threshold: 0, rootMargin },
  );
  io.observe(el);
  return () => {
    done = true;
    io.disconnect();
  };
}
