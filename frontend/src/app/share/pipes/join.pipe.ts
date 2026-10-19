import { Pipe, PipeTransform } from '@angular/core';

function getByPath(obj: any, path: string): any {
  return path.split('.').reduce((acc, key) => acc?.[key], obj);
}

@Pipe({
  name: 'join',
  standalone: true,
})
export class JoinPipe implements PipeTransform {
  transform(value: any[], separator: string = ', ', itemPath: string = ''): string {
    if (itemPath) {
      return value?.map((item: any) => getByPath(item, itemPath))?.join(separator) || '';
    }
    return value?.join(separator) || '';
  }
}
