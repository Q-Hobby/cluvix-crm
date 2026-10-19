import { Service } from '@angular/core';
import { Observable, catchError, concat, EMPTY, map, of, tap } from 'rxjs';

/** TTL khớp Redis cache BE của dashboard (300s). */
export const CRM_DASHBOARD_DATA_TTL_MS = 5 * 60 * 1000;

export interface CachedResult<T> {
  value: T;
  /** true = số cũ từ bộ nhớ (đang làm mới nền); false = số vừa lấy từ API. */
  stale: boolean;
}

/**
 * Cache số liệu khối Dashboard CRM trong BỘ NHỚ PHIÊN (epic crm-dashboard-customizable, PRD FR19).
 * Quay lại dashboard trong TTL ⇒ hiện ngay số cũ rồi làm mới nền (stale-while-revalidate).
 *
 * ⚠ KHÔNG ghi gì xuống localStorage/sessionStorage/IndexedDB: số doanh thu không được nằm lại trên máy
 * dùng chung ở quầy. Khoá do caller dựng và PHẢI gồm bộ lọc (`<widgetKey>|<from>|<to>|<branchId>`). CRM: 1 DB =
 * 1 thương hiệu ⇒ không kèm công ty như ERP. Xoá sạch khi logout (AuthService.clearAllAuthData) ⇒ tài khoản sau
 * trên cùng tab không thấy số của tài khoản trước.
 */
@Service()
export class CrmDashboardDataCache {
  private readonly store = new Map<string, { value: unknown; at: number }>();

  /** Phát số cũ (nếu còn hạn) rồi số mới; lỗi khi đã có số cũ thì giữ số cũ (interceptor đã toast). */
  fetch<T>(key: string, loader: () => Observable<T>, now: () => number = Date.now): Observable<CachedResult<T>> {
    const hit = this.store.get(key);
    const fresh$ = loader().pipe(
      tap((value) => this.store.set(key, { value, at: now() })),
      map((value): CachedResult<T> => ({ value, stale: false })),
    );
    if (hit && now() - hit.at < CRM_DASHBOARD_DATA_TTL_MS) {
      return concat(of({ value: hit.value as T, stale: true }), fresh$.pipe(catchError(() => EMPTY)));
    }
    return fresh$;
  }

  clear(): void {
    this.store.clear();
  }
}
