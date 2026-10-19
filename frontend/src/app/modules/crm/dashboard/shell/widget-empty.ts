/**
 * Điều kiện "rỗng" của các khối theo đúng giá trị BE trả khi không có dữ liệu (`crm/dashboard/handler/handler.go`):
 * `[]` (danh sách), `{}` (marketing-view khi chưa cấu hình nguồn online), `{"totalComputed":0}` (CLV).
 */
export const isEmptyList = (v: unknown): boolean => !Array.isArray(v) || v.length === 0;
export const isEmptyObject = (v: unknown): boolean => !v || typeof v !== 'object' || Object.keys(v).length === 0;
export const isEmptyClv = (v: { totalComputed?: number } | null | undefined): boolean => !v?.totalComputed;
export const isEmptyChartRows = (v: { data?: unknown[] } | null | undefined): boolean => !v?.data?.length;
/**
 * Danh sách name/value RỖNG hoặc TOÀN 0 — BE trả đủ tầng mà mọi value = 0 (vd estimate-price khi có ngân sách nhưng phễu
 * = 0; status-overview khi khách trong kỳ chỉ ở trạng thái ngoài phễu) ⇒ vẽ phễu toàn "0" vô nghĩa (QA story-08 P1).
 */
export const isEmptyRows = (v: unknown): boolean =>
  !Array.isArray(v) || v.every((r) => !Number((r as { value?: unknown })?.value));
/** Thống kê dịch vụ rỗng hoặc mọi dòng quantity = 0 và revenue = 0. */
export const isEmptyServiceStats = (v: unknown): boolean =>
  !Array.isArray(v) ||
  v.every((r) => !Number((r as { quantity?: unknown })?.quantity) && !Number((r as { revenue?: unknown })?.revenue));
