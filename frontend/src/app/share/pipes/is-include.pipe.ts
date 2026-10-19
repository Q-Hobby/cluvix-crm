import { Pipe, PipeTransform } from '@angular/core';

@Pipe({
  name: 'isInclude',
  standalone: true,
})
export class IsIncludePipe implements PipeTransform {
  transform(value: any[], checkItem: any): boolean {
    return value?.includes(checkItem) ?? false;
  }
}
