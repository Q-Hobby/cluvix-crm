import { Pipe, PipeTransform } from '@angular/core';

@Pipe({
  name: 'avatarWord',
  standalone: true,
})
export class AvatarWordPipe implements PipeTransform {
  transform(value: string): string | null {
    if (value) {
      const arrayName = value.trim().split(' ');
      return arrayName[arrayName.length - 1][0].toUpperCase();
    }
    return null;
  }
}
