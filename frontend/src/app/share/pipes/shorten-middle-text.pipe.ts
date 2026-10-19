import { Pipe, PipeTransform } from '@angular/core';

@Pipe({ name: 'shortenMiddleText', standalone: true })
export class ShortenMiddleTextPipe implements PipeTransform {
  transform(value: string, start = 8, end = 10): string {
    if (!value || value.length <= start + end) return value;
    return value.substring(0, start) + '.....' + value.substring(value.length - end);
  }
}
