import { Pipe, PipeTransform } from '@angular/core';

@Pipe({
  name: 'phoneFormat',
  standalone: true,
})
export class PhoneFormatPipe implements PipeTransform {
  transform(value: string): string {
    if (value) {
      return value.replace(/^(\d{4})(\d{3})(\d{3})$/g, '$1 $2 $3');
    }
    return '';
  }
}
