import { Pipe, PipeTransform } from '@angular/core';

@Pipe({
  standalone: true,
  name: 'isNegative',
})
export class IsNegativePipe implements PipeTransform {
  transform(value: number): boolean {
    return value < 0;
  }
}
