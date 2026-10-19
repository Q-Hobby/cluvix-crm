import { Pipe, PipeTransform } from '@angular/core';

@Pipe({
  name: 'trimText',
  standalone: true,
})
export class TrimTextPipe implements PipeTransform {
  transform(value: any) {
    if (!value) {
      return '';
    }

    return value.trim();
  }
}
