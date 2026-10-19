import { Pipe, PipeTransform } from '@angular/core';

@Pipe({
  name: 'oldNewType',
  standalone: true,
})
export class OldNewTypePipe implements PipeTransform {
  transform(value: number | string | null | undefined): string {
    const map: Record<number | string, string> = {
      1: 'Đơn hàng mới',
      2: 'Đơn hàng cũ',
    };
    return map[value ?? ''] ?? 'Không xác định';
  }
}
