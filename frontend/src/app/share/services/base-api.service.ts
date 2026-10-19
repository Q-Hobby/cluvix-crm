import { inject, Service, signal } from '@angular/core';
import { HttpClient, HttpParams, HttpXhrBackend } from '@angular/common/http';
import { DatePipe } from '@angular/common';
import { format, parse } from 'date-fns';
import { Observable } from 'rxjs';
import { map } from 'rxjs/operators';

import { ApiService } from './api.service';
import { toFormData } from './utils.service';

import { ListApiResponse } from '../models/list-api-response';
import { ApiResponseOneModel } from '@/core/interface/ApiResponseOneModel';
import { Identifiable } from '@/core/interface/identifiable';
import { catchApiError } from '@/share/helpers/catch-api-error';
import { TimezoneService } from '@/core/services/timezone.service';

@Service()
export abstract class BaseApiService<T extends Identifiable> {
  title?: string;
  model = signal<T | null>(null);
  loading = signal<boolean>(false);
  protected readonly apiService = inject(ApiService);
  protected readonly timezoneService = inject(TimezoneService);
  // ⚠️ Chỉ dùng cho upload có progress (fetch không có progress event)
  protected readonly xhrHttp = new HttpClient(inject(HttpXhrBackend));
  protected apiModule = 'crm';
  protected endpoint = '';

  protected get path(): string {
    return `${this.apiModule}/${this.endpoint}`;
  }
  protected dateFields: any = [];
  protected multiSelectFields: any = [];
  protected unixTimesTampFields: any = [];
  protected autoCompleteFields: any = [];
  protected autoCompleteFieldsMultiple: any = [];
  protected jsonFields: any = [];
  protected dateTimeFields: any = [];

  // ----------------- CRUD -----------------
  create<P extends object>(
    createParams: P,
    options: any = {},
    isToFormData = false,
  ): Observable<P> {
    const params = this.beforeSave(createParams);
    return this.apiService.post<{ data: P }>(
      `${this.path}`,
      isToFormData ? toFormData(params) : params,
      options,
    ).pipe(
      map((res) => (res?.data ?? res) as P),
    );
  }

  update<P extends object>(
    id: number,
    updateParams: P,
    options: any = {},
    isToFormData = false,
  ): Observable<ApiResponseOneModel<P>> {
    const params = this.beforeSave(updateParams);
    return this.apiService
      .put<
        ApiResponseOneModel<P>
      >(`${this.path}/${id}`, isToFormData ? toFormData(params) : params, options)
      .pipe(
        map((res) => {
          if (res?.data) {
            const cloned = JSON.parse(JSON.stringify(res.data));
            res.data = this.afterLoad(cloned) as P;
          }
          return res;
        }),
      );
  }

  get(id: number | string, options: any = {}): Observable<T> {
    return this.apiService.get<{ data: T }>(`${this.path}/${id}`, options).pipe(
      map((res) => {
        const raw = res?.data ?? res;
        const cloned = JSON.parse(JSON.stringify(raw));
        return this.afterLoad(cloned) as T;
      }),
    );
  }

  list(params: any = {}): Observable<ListApiResponse<T>> {
    // gộp query chuẩn bằng HttpParams để tương thích HttpClient
    const query: Record<string, any> = { ...(params?.params ?? {}) };

    const pageFromQuery = Number(query.page);
    const pageFromTop = Number(params?.page);
    query.page = pageFromQuery > 0 ? pageFromQuery : pageFromTop > 0 ? pageFromTop : 1;

    const httpParams = new HttpParams({ fromObject: query });
    const opts = { ...params, params: httpParams };

    return this.apiService.get<ListApiResponse<T>>(this.path, opts).pipe(
      map((res) => {
        // BE có lỗi hiếm gặp trả 200 kèm body rỗng → res parse ra null, tránh vỡ cả list.
        if (!res) return { items: [] } as ListApiResponse<T>;
        const cloned = JSON.parse(JSON.stringify(res));
        cloned.items?.forEach((item) => this.afterLoad(item));
        return cloned;
      }),
    );
  }

  delete(id: number): Observable<string> {
    return this.apiService.delete(`${this.path}/${id}`);
  }

  loadByKeyWord(keyword: string, params: any = {}): Observable<any> {
    // xây query bằng params thay vì nối chuỗi
    const httpParams = new HttpParams({
      fromObject: { ...(params?.params ?? {}), keyword: String(keyword ?? '') },
    });
    const opts = { ...params, params: httpParams };
    return this.apiService.get(`${this.path}/load-by-keyword`, opts);
  }

  // Form default model
  getInitialObject() {
    return {};
  }

  exportData() {
    // nếu BE trả file trực tiếp, cân nhắc dùng responseType: 'blob'
    return this.apiService.post(`${this.path}/export-data-excel`, {});
  }

  importData(file: File) {
    const formData = new FormData();
    formData.append('file', file, file.name);

    // ⚠️ Dùng XHR để có progress (fetch không có)
    return this.xhrHttp
      .post(`${this.path}/import-data`, formData, {
        reportProgress: true,
        observe: 'events',
      })
      .pipe(catchApiError());
  }

  // ----------------- Transform trước khi SAVE -----------------
  protected beforeSave<P extends object>(saveParams: P): P {
    const params = { ...(saveParams as any) } as Record<string, any>;

    // Date -> 'yyyy-MM-dd'
    this.dateFields.forEach((field) => {
      const v = params[field];
      if (!v) return;
      if (v instanceof Date) {
        if (isNaN(v.getTime())) return;
        params[field] = format(v, 'yyyy-MM-dd');
      } else if (typeof v === 'string' || typeof v === 'number') {
        const d = new Date(v);
        if (!isNaN(d.getTime())) params[field] = format(d, 'yyyy-MM-dd');
      }
    });

    this.dateTimeFields.forEach((field) => {
      const v = params[field];
      if (!v) return;
      if (v instanceof Date) {
        if (isNaN(v.getTime())) return;
        params[field] = format(v, 'yyyy-MM-dd HH:mm:ss');
      }
    });

    // Date/number/string -> unix(s)
    this.unixTimesTampFields.forEach((field) => {
      const v = params[field];
      if (!v) return;
      if (v instanceof Date) {
        params[field] = this.timezoneService.dateHmsToUnix(v, v.getHours(), v.getMinutes(), v.getSeconds());
      } else if (typeof v === 'string' || typeof v === 'number') {
        const d = new Date(v);
        if (!isNaN(d.getTime())) params[field] = this.timezoneService.dateHmsToUnix(d, d.getHours(), d.getMinutes(), d.getSeconds());
      }
    });

    // autocomplete object -> key
    this.autoCompleteFields.forEach((field) => {
      const v = params[field];
      if (!v) return;
      const key = this.autoCompleteFields[field] ?? 'value';
      params[field] = v?.[key];
    });

    // autocomplete array -> key[]
    this.autoCompleteFieldsMultiple.forEach((field) => {
      const arr = params[field];
      if (!Array.isArray(arr)) return;
      const key = this.autoCompleteFields[field] ?? 'value';
      params[field] = arr.map((row: any) => row?.[key]);
    });

    // jsonFields: nếu tất cả phần tử trống -> null
    this.jsonFields.forEach((field) => {
      const arr = params[field];
      if (!Array.isArray(arr) || arr.length === 0) return;
      const keys = Object.keys(arr[0] ?? {});
      let allEmpty = true;
      outer: for (const row of arr) {
        for (const k of keys) {
          if (row?.[k] !== '' && row?.[k] !== null && row?.[k] !== undefined) {
            allEmpty = false;
            break outer;
          }
        }
      }
      if (allEmpty) params[field] = null;
    });

    // string[] -> CSV
    this.multiSelectFields.forEach((field) => {
      const v = params[field];
      if (Array.isArray(v)) params[field] = v.join(',');
    });

    return params as P;
  }

  // ----------------- Transform sau khi LOAD -----------------
  protected afterLoad<U>(model: U | null): U | null {
    if (!model) return null;
    const m = model as Record<string, any>;

    // 'yyyy-MM-dd' -> Date (slice(0,10) handles ISO datetime strings like '1992-05-04T00:00:00Z')
    this.dateFields.forEach((field: string) => {
      if (field.includes('.')) {
        const v = this.getNestedValue(m, field);
        if (v)
          this.setNestedValue(m, field, parse(String(v).slice(0, 10), 'yyyy-MM-dd', this.timezoneService.now()));
      } else {
        const v = m[field];
        m[field] = v ? parse(String(v).slice(0, 10), 'yyyy-MM-dd', this.timezoneService.now()) : v;
      }
    });

    // 'yyyy-MM-dd HH:mm:ss' -> Date
    this.dateTimeFields.forEach((field: string) => {
      if (field.includes('.')) {
        const v = this.getNestedValue(m, field);
        if (v) this.setNestedValue(m, field, parse(String(v), 'yyyy-MM-dd HH:mm:ss', new Date()));
      } else {
        const v = m[field];
        m[field] = v ? parse(String(v), 'yyyy-MM-dd HH:mm:ss', new Date()) : v;
      }
    });

    // unix(s) -> Date
    this.unixTimesTampFields.forEach((field: string) => {
      if (field.includes('.')) {
        const v = this.getNestedValue(m, field);
        if (v && Number(v) > 0) this.setNestedValue(m, field, new Date(Number(v) * 1000));
      } else {
        const v = m[field];
        if (v && Number(v) > 0) m[field] = new Date(Number(v) * 1000);
      }
    });

    // CSV -> string[]
    this.multiSelectFields.forEach((field: string) => {
      if (field.includes('.')) {
        const v = this.getNestedValue(m, field);
        if (typeof v === 'string') this.setNestedValue(m, field, v.split(','));
      } else {
        const v = m[field];
        m[field] = typeof v === 'string' ? v.split(',') : v;
      }
    });

    return m as U;
  }

  // ----------------- LABEL HELPERS -----------------
  /**
   * Format unix timestamp to readable date string
   * @param timestamp - Unix timestamp or Date object
   * @param withTime - Include time (default: true)
   * @returns Formatted date string
   */
  protected formatTimestampFields(
    timestamp: number | Date | null | undefined,
    withTime: boolean = true,
  ): string {
    // Xử lý các trường hợp không hợp lệ
    if (!timestamp || timestamp === 0) return '';

    let date: Date;
    if (timestamp instanceof Date) {
      date = timestamp;
    } else if (typeof timestamp === 'number') {
      // Nếu là timestamp (giây hoặc milliseconds)
      // Timestamp hợp lệ phải > 0 và trong khoảng hợp lý (sau 1970)
      if (timestamp <= 0) return '';
      date = timestamp > 10000000000 ? new Date(timestamp) : new Date(timestamp * 1000);

      // Chỉ kiểm tra năm cho unix timestamp
      if (date.getFullYear() < 1970 || date.getFullYear() > 2100) {
        return '';
      }
    } else {
      return '';
    }

    // Kiểm tra date hợp lệ chung
    const time = date.getTime();
    if (isNaN(time)) {
      return '';
    }

    const datePipe = new DatePipe('en-US');
    const format = withTime ? 'dd/MM/yyyy HH:mm' : 'dd/MM/yyyy';

    return datePipe.transform(date, format) || '';
  }

  /**
   * Add formatted timestamp fields to item without overwriting originals
   * @param item - Item to add formatted fields to
   * @param fields - Array of timestamp field names to format (supports nested: 'customer.created_at')
   * @param dateOnlyFields - Array of field names that should be formatted without time
   * @returns Item with additional formatted fields (fieldNameFormatted)
   */
  protected addFormattedTimestampFields<T>(item: T, fields: string[], dateOnlyFields: string[] = []): T {
    if (!item || !Array.isArray(fields)) return item;

    const result = { ...item } as any;
    fields.forEach((field) => {
      const value = this.getNestedValue(result, field);
      const formattedField = field.replace(/\./g, '_') + 'Formatted';
      const withTime = !dateOnlyFields.includes(field);
      result[formattedField] = value ? this.formatTimestampFields(value, withTime) : '';
    });

    return result;
  }

  /**
   * Get nested value from object using dot notation
   * @param obj - Object to get value from
   * @param path - Dot notation path (e.g., 'customer.created_at')
   * @returns Value at path or undefined
   */
  private getNestedValue(obj: any, path: string): any {
    return path.split('.').reduce((current, key) => current?.[key], obj);
  }

  /**
   * Set nested value in object using dot notation
   * @param obj - Object to set value in
   * @param path - Dot notation path (e.g., 'customer.created_at')
   * @param value - Value to set
   */
  private setNestedValue(obj: any, path: string, value: any): void {
    const keys = path.split('.');
    const lastKey = keys.pop()!;
    const target = keys.reduce((current, key) => {
      if (!current[key]) current[key] = {};
      return current[key];
    }, obj);
    target[lastKey] = value;
  }
}
