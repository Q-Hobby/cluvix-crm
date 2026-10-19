import { inject, Pipe, PipeTransform } from '@angular/core';
import { DomSanitizer } from '@angular/platform-browser';

import { sanitizeUserHtml } from './sanitize-user-html';

/**
 * Render HTML do người dùng nhập. Logic làm sạch nằm ở `sanitize-user-html.ts` (file thuần, không
 * phụ thuộc Angular) để test chạy được không cần TestBed — xem `sanitize-user-html.spec.ts`.
 */
@Pipe({
  name: 'safeHtml',
  standalone: true,
})
export class SafeHtmlPipe implements PipeTransform {
  private sanitized = inject(DomSanitizer);

  transform(value: any) {
    return this.sanitized.bypassSecurityTrustHtml(sanitizeUserHtml(value));
  }
}
