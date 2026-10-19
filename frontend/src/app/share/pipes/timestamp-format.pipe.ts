import { Pipe, PipeTransform } from '@angular/core';

@Pipe({
  name: 'timestampFormat',
  standalone: true,
})
export class TimestampFormatPipe implements PipeTransform {
  transform(value: any): string {
    if (!value) return '—';

    let timestamp: number;

    if (value instanceof Date) {
      if (isNaN(value.getTime())) return '—';
      timestamp = Math.floor(value.getTime() / 1000);
    } else {
      timestamp = Number(value);
    }

    if (!timestamp || timestamp <= 0) return '—';

    const date = new Date(timestamp * 1000);
    if (isNaN(date.getTime())) return '—';

    const day = String(date.getDate()).padStart(2, '0');
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const year = date.getFullYear();
    const hours = String(date.getHours()).padStart(2, '0');
    const minutes = String(date.getMinutes()).padStart(2, '0');

    return `${day}-${month}-${year} ${hours}:${minutes}`;
  }
}
