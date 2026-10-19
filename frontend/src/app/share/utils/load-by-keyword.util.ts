/**
 * D110 — đọc phản hồi của `GET /crm/customer/load-by-keyword` khi người gọi hỏi `?with_total=1`.
 *
 * ⛔ Vì sao phải có hàm riêng chứ không `res.items` thẳng: endpoint này có HAI hình dạng phản hồi và
 * BE cố ý giữ vậy —
 *   · KHÔNG có `with_total` ⇒ **MẢNG TRẦN** (mặc định, 12 chỗ gọi FE khác đang đọc kiểu này);
 *   · CÓ `with_total=1`  ⇒ `{ items, total }`.
 * Kiểu trả về của `BaseApiService.loadByKeyWord` là `any`, nên `tsc` KHÔNG bắt được nếu đọc nhầm
 * hình dạng — hỏng sẽ là màn hình rỗng câm lúc chạy. Hai hàm dưới chấp nhận CẢ HAI hình dạng nên chỗ
 * gọi vẫn chạy đúng khi nói chuyện với BE cũ (fail-safe), thay vì nổ.
 *
 * ⚠ `readKeywordTotal` trả `null` = **KHÔNG BIẾT TỔNG**, khác hẳn `0` = "không khớp khách nào". Chỗ
 * gọi PHẢI phân biệt hai thứ này: coi `null` là `0` sẽ kết luận "hết khách rồi" ngay cả khi danh
 * sách đang đầy.
 */

/** Lấy MẢNG dòng ra khỏi phản hồi, chấp nhận cả mảng trần lẫn `{items}`. Shape lạ ⇒ `[]`. */
export function readKeywordRows(res: unknown): unknown[] {
  if (Array.isArray(res)) return res;
  const items = (res as { items?: unknown } | null)?.items;
  return Array.isArray(items) ? items : [];
}

/** Lấy TỔNG số bản ghi khớp. `null` khi phản hồi không mang tổng (BE cũ / mảng trần / giá trị hỏng). */
export function readKeywordTotal(res: unknown): number | null {
  if (!res || Array.isArray(res) || typeof res !== 'object') return null;
  const raw = (res as { total?: unknown }).total;
  // ⛔ CHỈ chấp nhận kiểu `number` thật. Đừng nới thành `Number(raw)`: `Number('')` và `Number(null)`
  // đều ra `0` ⇒ một `total` khuyết bị đọc thành "0 khách khớp", tức kết luận NGƯỢC hoàn toàn.
  if (typeof raw !== 'number' || !Number.isFinite(raw) || raw < 0) return null;
  return Math.trunc(raw);
}
