import { Pipe, PipeTransform } from '@angular/core';

@Pipe({
  name: 'toArray',
  standalone: true,
})
export class ToArrayPipe implements PipeTransform {
  transform(value: number): Array<number> {
    return new Array(value).fill(0).map((n: any, index: any) => index + 1);
  }
}
