import {
  HttpClient,
  HttpContext,
  HttpErrorResponse,
  HttpHeaders,
  HttpParams,
} from '@angular/common/http';
import { inject, Service} from '@angular/core';
import { environment } from '@env/environment';
import { lastValueFrom, Observable, throwError } from 'rxjs';
import { catchError } from 'rxjs/operators';

import {
  DEDUPE_DISABLE,
  DEDUPE_KEY_SUFFIX,
  DEDUPE_TTL_MS,
} from '@/core/interceptors/api-dedupe.interceptor';

export type ApiResponseType = 'json' | 'blob' | 'text';

/**
 * KHÔNG có `observe` ở đây — và đừng thêm lại. `ApiService` luôn trả BODY (`Observable<T>`), còn
 * `observe: 'events'`/`'response'` trả `Observable<HttpEvent<T>>` nên đổi hẳn kiểu trả về và làm
 * `next` bắn nhiều lần (mỗi sự kiện upload một lần). Field này từng có mặt trong interface nhưng bị
 * `createHttpOptions` vứt đi, nên 13 chỗ `importData` tưởng mình đang nhận tiến độ upload trong khi
 * thực tế chỉ nhận body — khai báo mà không có hiệu lực còn tệ hơn không khai báo.
 * Cần event thật (thanh tiến độ, đọc header response) → tiêm thẳng `HttpClient` như
 * `header-sync.service.ts` / `teamchat.service.ts#uploadFile` đang làm. `reportProgress` cũng đã bỏ
 * theo: không có `observe: 'events'` thì nó không sinh ra event nào.
 */
export interface ApiOptions {
  withCredentials?: boolean;
  headers?: HttpHeaders;
  params?: HttpParams;
  responseType?: ApiResponseType;

  context?: HttpContext; // nếu muốn tự cấp context thủ công
  dedupeDisable?: boolean; // tắt dedupe/micro-cache cho request này
  dedupeTtlMs?: number; // TTL riêng (ms)
  dedupeKeySuffix?: string; // nối key (ví dụ theo branchId)
}

@Service()
export class ApiService {
  readonly baseUrl = environment.apiUrl.replace(/\/+$/, '');
  private http = inject(HttpClient);

  // 🧠 Observable version
  get<T = any>(path: string, options: ApiOptions = {}): Observable<T> {
    return this.wrap(this.http.get<T>(this.fullUrl(path), this.createHttpOptions(options)));
  }

  getLink<T = any>(url: string, options: ApiOptions = {}): Observable<T> {
    return this.wrap(this.http.get<T>(url, this.createHttpOptions(options)));
  }

  post<T = any>(path: string, body: Record<string, any> = {}, options: ApiOptions = {}): Observable<T> {
    return this.wrap(this.http.post<T>(this.fullUrl(path), body, this.createHttpOptions(options)));
  }

  put<T = any>(path: string, body: Record<string, any> = {}, options: ApiOptions = {}): Observable<T> {
    return this.wrap(this.http.put<T>(this.fullUrl(path), body, this.createHttpOptions(options)));
  }

  // story-27: BE teamchat/org-channel dùng PATCH (con trỏ optional = "không đụng tới" — khác `put`
  // ghi đè toàn bộ record). KHÔNG dùng cho endpoint nào khác trừ khi BE thật sự trả PATCH.
  patch<T = any>(path: string, body: Record<string, any> = {}, options: ApiOptions = {}): Observable<T> {
    return this.wrap(this.http.patch<T>(this.fullUrl(path), body, this.createHttpOptions(options)));
  }

  delete<T = any>(path: string, options: ApiOptions = {}): Observable<T> {
    return this.wrap(this.http.delete<T>(this.fullUrl(path), this.createHttpOptions(options)));
  }

  // 🔁 Async/await version
  async getAsync<T>(path: string, options: ApiOptions = {}): Promise<T> {
    return await lastValueFrom(this.get<T>(path, options));
  }

  async postAsync<T>(
    path: string,
    body: Record<string, any> = {},
    options: ApiOptions = {},
  ): Promise<T> {
    return await lastValueFrom(this.post<T>(path, body, options));
  }

  async putAsync<T>(
    path: string,
    body: Record<string, any> = {},
    options: ApiOptions = {},
  ): Promise<T> {
    return await lastValueFrom(this.put<T>(path, body, options));
  }

  async deleteAsync<T>(path: string, options: ApiOptions = {}): Promise<T> {
    return await lastValueFrom(this.delete<T>(path, options));
  }

  private fullUrl(path: string): string {
    if (/^https?:\/\//i.test(path)) return path;
    const base = this.baseUrl.replace(/\/+$/, '');
    const p = String(path).replace(/^\/+/, '');
    return `${base}/${p}`;
  }

  // Nhận `Observable<any>`: options truyền cho HttpClient là object động (xem createHttpOptions) nên
  // TypeScript chọn overload trả `HttpEvent<T>`. Runtime luôn là body vì `observe` cố định 'body',
  // và T là kiểu do caller khai — `get<Blob>` ra `Observable<Blob>`.
  private wrap<T>(obs$: Observable<any>): Observable<T> {
    return obs$.pipe(catchError(this.handleError)) as Observable<T>;
  }

  private handleError(error: HttpErrorResponse) {
    if (!environment.prod) {
      console.error('❌ API Error:', error);
    }
    // Giữ nguyên HttpErrorResponse để interceptor đọc được status code
    return throwError(() => error);
  }

  /**
   * Dựng options cho `HttpClient`.
   *
   * `responseType` PHẢI đi theo caller: trước đây hàm này hardcode `'json'` và im lặng vứt bỏ
   * `options.responseType`, nên `get<Blob>(..., { responseType: 'blob' })` vẫn đi đường JSON —
   * Angular đem file .xlsx đi `JSON.parse` và ném `Unexpected token 'P'` ("PK" là magic bytes của
   * zip). Vì lỗ này mà mọi chỗ cần tải file đều phải bỏ qua `ApiService` để gọi thẳng `HttpClient`.
   *
   * Kiểu trả về để `any`: overload của `HttpClient` chọn theo LITERAL type của `responseType`, mà ở
   * đây nó là union runtime. `ApiService` cố ý luôn trả `Observable<T>` với T do caller khai
   * (`get<Blob>`), nên bỏ qua phần chọn overload đó là đúng chủ đích, không phải né lỗi kiểu.
   */
  private createHttpOptions(options: ApiOptions = {}): any {
    // build context
    let ctx = options.context ?? new HttpContext();
    if (options.dedupeDisable !== undefined) ctx = ctx.set(DEDUPE_DISABLE, !!options.dedupeDisable);
    if (options.dedupeTtlMs !== undefined)
      ctx = ctx.set(DEDUPE_TTL_MS, Number(options.dedupeTtlMs));
    if (options.dedupeKeySuffix) ctx = ctx.set(DEDUPE_KEY_SUFFIX, String(options.dedupeKeySuffix));

    return {
      headers: options.headers,
      params: options.params,
      observe: 'body',
      responseType: options.responseType ?? 'json',
      withCredentials: options.withCredentials ?? true,
      context: ctx,
    };
  }
}
