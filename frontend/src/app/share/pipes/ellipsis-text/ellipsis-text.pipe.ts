import { Pipe, PipeTransform } from '@angular/core';

@Pipe({
  name: 'ellipsisText',
  standalone: true,
})
export class EllipsisTextPipe implements PipeTransform {
  transform(value: string, length: any = 80, ramp: any = '...', delimiter: any = ' ') {
    if (!value) {
      return '';
    }
    const txt = value.toString();
    if (txt.length < length) {
      return value;
    }
    const text = txt.substr(0, length);
    const lastIndex = text.lastIndexOf(delimiter);
    if (lastIndex > 0) {
      return text.substr(0, lastIndex) + ' ' + ramp;
    }
    return value.substr(0, length) + ' ' + ramp;
  }
}
