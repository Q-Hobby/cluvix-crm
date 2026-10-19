import { inject, Pipe, PipeTransform } from '@angular/core';

import { TimezoneService } from '@/core/services/timezone.service';

@Pipe({
  name: 'compareDate',
  standalone: true,
})
export class CompareDatePipe implements PipeTransform {
  private tz = inject(TimezoneService);
  transform(date1: number, date2: number | null): boolean {
    const ref =
      date2 ??
      (() => {
        const d = this.tz.now();
        d.setHours(0, 0, 0, 0);
        return d.getTime();
      })();

    return date1 > ref;
  }
}
