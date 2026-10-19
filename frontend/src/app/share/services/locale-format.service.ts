import { computed, Service} from '@angular/core';
import { authBusinessProfile } from '@/core/services/auth/auth.state';

const DEFAULT_LOCALE = 'vi-VN';
const DEFAULT_CURRENCY = 'VND';

@Service()
export class LocaleFormatService {
  readonly currentLocale = computed<string>(() => authBusinessProfile()?.currency_locale || DEFAULT_LOCALE);
  readonly currencyCode = computed<string>(() => authBusinessProfile()?.currency || DEFAULT_CURRENCY);

  formatDate(d: Date | number, opts?: Intl.DateTimeFormatOptions): string {
    const date = typeof d === 'number' ? new Date(d < 1e12 ? d * 1000 : d) : d;
    return date.toLocaleDateString(this.currentLocale(), opts);
  }

  formatTime(d: Date | number, opts?: Intl.DateTimeFormatOptions): string {
    const date = typeof d === 'number' ? new Date(d < 1e12 ? d * 1000 : d) : d;
    return date.toLocaleTimeString(this.currentLocale(), opts);
  }

  formatNumber(value: number, opts?: Intl.NumberFormatOptions): string {
    return new Intl.NumberFormat(this.currentLocale(), opts).format(value);
  }
}
