import {Service, signal } from '@angular/core';

@Service()
export class LocaleService {
  readonly currentLocale = signal<string>('vi-VN');

  setLocale(locale: string) {
    this.currentLocale.set(locale);
  }

  getLocale(): string {
    return this.currentLocale();
  }
}
