import { Pipe, PipeTransform } from '@angular/core';

@Pipe({
  name: 'formatMonth',
  standalone: true,
  pure: true,
})
export class FormatMonthPipe implements PipeTransform {
  transform(month: number): string {
    if (!month) return '';
    const s = month.toString();
    return s.substring(4) + '/' + s.substring(0, 4);
  }
}
