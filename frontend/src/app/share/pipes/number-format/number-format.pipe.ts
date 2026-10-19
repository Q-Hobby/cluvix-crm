import { DecimalPipe } from '@angular/common';
import { Pipe, PipeTransform } from '@angular/core';

@Pipe({
  name: 'numberFormat',
  standalone: true,
})
export class NumberFormatPipe implements PipeTransform {
  transform(value: any): any {
    const decimalPipe = new DecimalPipe('en-US');
    return decimalPipe.transform(value, '1.0');
  }
}
