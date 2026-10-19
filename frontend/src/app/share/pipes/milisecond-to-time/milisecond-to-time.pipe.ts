import { Pipe, PipeTransform } from '@angular/core';

@Pipe({
  name: 'milisecondToTime',
  standalone: true,
})
export class MilisecondToTimePipe implements PipeTransform {
  transform(duration: number | string): string | number {
    let milliseconds: number = typeof duration === 'number' ? duration : Number(duration);
    if (isNaN(milliseconds)) {
      return duration;
    }
    const converter = (num: number) => (num > 9 ? num.toString() : `0${num}`);
    let minutes: number, seconds: number;
    seconds = Math.floor(milliseconds / 1000);
    minutes = Math.floor(seconds / 60);
    seconds = seconds % 60;
    const hours = Math.floor(minutes / 60);
    minutes = minutes % 60;
    return `${converter(hours)}:${converter(minutes)}:${converter(seconds)}`;
  }
}
