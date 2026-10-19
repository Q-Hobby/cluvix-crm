/**
 * Rút gọn NHÃN hiển thị của biểu đồ (legend, nhãn lát, nhãn trục) — dùng trong `formatter`, KHÔNG cắt dữ liệu: tên đầy đủ
 * vẫn nằm trong `data` nên tooltip hiện đủ (cắt sẵn trong data thì không còn cách nào xem tên đầy đủ — /check-ui 2026-10-09).
 * `max <= 0` ⇒ giữ nguyên.
 */
export function truncateLabel(name: string | null | undefined, max: number): string {
  const s = name ?? '';
  return max > 0 && s.length > max ? s.substring(0, max) + '…' : s;
}
