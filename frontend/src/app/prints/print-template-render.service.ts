import { inject, Service} from '@angular/core';
import { HttpParams } from '@angular/common/http';
import { Observable, of } from 'rxjs';
import { catchError, map } from 'rxjs/operators';
import { ApiService } from '@/share/services/api.service';
import { PrintTemplateModel } from '@/core/models/config/print-template.model';

/**
 * Service dùng trong các print pages để:
 * 1. Fetch template mặc định theo type (có cache trong-session)
 * 2. Render HTML template với data thực tế (Handlebars-lite engine)
 *
 * Mỗi print page gọi getDefault() → nếu có template → render(),
 * nếu không có → fall back layout hardcode.
 */
@Service()
export class PrintTemplateRenderService {
  private api = inject(ApiService);
  // Cache trong session: `${type}:${branchId}` → template | null
  private readonly cache = new Map<string, PrintTemplateModel | null>();

  /**
   * Lấy template mặc định cho type + branch.
   * Trả null nếu chưa cấu hình (print page sẽ dùng layout mặc định).
   */
  getDefault(type: string, branchId?: number): Observable<PrintTemplateModel | null> {
    const cacheKey = `${type}:${branchId ?? 0}`;
    if (this.cache.has(cacheKey)) {
      return of(this.cache.get(cacheKey) ?? null);
    }

    let params = new HttpParams().set('type', type);
    if (branchId) params = params.set('branch_id', String(branchId));

    return this.api.get<{ data: PrintTemplateModel }>('config/print-template/default', { params }).pipe(
      map((res: any) => {
        // res?.data có thể là null (BE báo chưa cấu hình) — dùng != null thay vì ??
        const raw = res?.data !== undefined ? res.data : res;
        const item: PrintTemplateModel | null = raw != null ? raw : null;
        this.cache.set(cacheKey, item);
        return item;
      }),
      catchError(() => {
        this.cache.set(cacheKey, null);
        return of(null);
      }),
    );
  }

  /**
   * Render template HTML với data.
   * Hỗ trợ:
   *   - {{key.nested}}        — thay thế đơn giản
   *   - {{#each items}} ... {{this.key}} ... {{/each}}  — vòng lặp
   *   - {{#if condition}} ... {{/if}}  — điều kiện (truthy check)
   */
  render(template: PrintTemplateModel, data: Record<string, any>): string {
    if (!template.template_html) return '';
    return renderTemplate(template.template_html, data);
  }

  clearCache(): void {
    this.cache.clear();
  }
}

// ─── Template engine ──────────────────────────────────────────────────────────

export function renderTemplateHtml(html: string, data: Record<string, any>): string {
  return renderTemplate(html, data);
}

function renderTemplate(html: string, data: Record<string, any>): string {
  // 1. {{#each key}} ... {{/each}}
  html = html.replace(
    /\{\{#each\s+([\w.]+)\}\}([\s\S]*?)\{\{\/each\}\}/g,
    (_match, keyPath, innerTpl) => {
      const arr = resolvePath(data, keyPath);
      if (!Array.isArray(arr)) return '';
      return arr
        .map((item, idx) => renderTemplate(innerTpl, { ...data, this: item, '@index': idx }))
        .join('');
    },
  );

  // 2. {{#if key}} ... {{/if}}
  html = html.replace(
    /\{\{#if\s+([\w.]+)\}\}([\s\S]*?)\{\{\/if\}\}/g,
    (_match, keyPath, innerTpl) => {
      const val = resolvePath(data, keyPath);
      return val ? renderTemplate(innerTpl, data) : '';
    },
  );

  // 3. {{key.path}} — simple substitution
  html = html.replace(/\{\{([\w.@]+)\}\}/g, (_match, keyPath) => {
    const val = resolvePath(data, keyPath);
    return val !== undefined && val !== null ? String(val) : '';
  });

  return html;
}

function resolvePath(obj: any, path: string): any {
  return path.split('.').reduce((acc, key) => {
    if (acc === undefined || acc === null) return undefined;
    return acc[key];
  }, obj);
}
