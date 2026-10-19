import {Service, signal } from '@angular/core';

@Service()
export class ReminderAlertService {
  private count = signal<number>(0);

  getCount = this.count.asReadonly();

  setCount(value: number): void {
    this.count.set(value);
  }
}
