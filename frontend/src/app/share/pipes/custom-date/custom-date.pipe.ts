import { DatePipe } from '@angular/common';
import { Pipe, PipeTransform } from '@angular/core';

@Pipe({
  name: 'customDate',
  standalone: true,
})
export class CustomDatePipe implements PipeTransform {
  /**
   * Formats a Date or unix timestamp.
   * Auto-detects seconds vs milliseconds: values < 10^10 (≈ year 2286) treated as seconds.
   * Returns '' for null/0/undefined to avoid '01-01-1970' artifacts.
   */
  transform(value: Date | number | string | null | undefined, withTime: boolean = false): string {
    if (value === null || value === undefined || value === 0 || value === '') return '';

    let normalized: Date | number;
    if (typeof value === 'number') {
      normalized = value > 10000000000 ? value : value * 1000;
    } else if (typeof value === 'string') {
      const d = new Date(value);
      if (isNaN(d.getTime())) return '';
      normalized = d;
    } else {
      if (isNaN((value as Date).getTime())) return '';
      normalized = value;
    }

    const datePipe = new DatePipe('en-US');
    const format = withTime ? 'dd/MM/yyyy HH:mm' : 'dd/MM/yyyy';
    try {
      return datePipe.transform(normalized, format) ?? '';
    } catch {
      return '';
    }
  }
}
