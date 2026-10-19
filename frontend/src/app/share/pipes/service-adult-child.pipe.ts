import { Pipe, PipeTransform } from '@angular/core';

@Pipe({
  name: 'adultOrChild',
  standalone: true, // Nếu bạn dùng Angular standalone
})
export class ServiceAdultChildPipe implements PipeTransform {
  transform(value: number | string | null | undefined): string {
    const map: Record<number | string, string> = {
      1: 'Người lớn',
      2: 'Trẻ em',
    };
    return map[value ?? ''] ?? 'Không rõ';
  }
}
