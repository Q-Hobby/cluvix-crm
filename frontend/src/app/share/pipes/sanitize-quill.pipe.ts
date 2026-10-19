import { Pipe, PipeTransform } from '@angular/core';

@Pipe({
  standalone: true,
  name: 'sanitizeQuill',
})
export class SanitizeQuillPipe implements PipeTransform {
  transform(value: string | null | undefined): string {
    if (!value) return '';
    return value.replace(/&nbsp;/g, ' ');
  }
}
