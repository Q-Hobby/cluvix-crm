import { Pipe, PipeTransform } from '@angular/core';
import { authBusinessProfile } from '@/core/services/auth/auth.state';
import { formatCurrencyByBusinessProfile } from './currency-format.pipe';

@Pipe({
  name: 'currencyFormatBrief',
  standalone: true,
})
export class CurrencyFormatBriefPipe implements PipeTransform {
  transform(value: any): string {
    if (value === null || value === undefined || isNaN(value)) return '';

    const num = Number(value);
    const profile = authBusinessProfile();
    const locale = profile?.currency_locale || 'vi-VN';
    const symbol = profile?.currency_symbol || '₫';

    const fmt = (v: number, digits = 1) =>
      new Intl.NumberFormat(locale, { maximumFractionDigits: digits }).format(v);

    const abs = Math.abs(num);
    const sign = num < 0 ? '-' : '';

    // Ngưỡng rút gọn theo locale: vi-VN dùng Tỷ/Triệu, các locale khác dùng B/M/K
    const isVi = locale.startsWith('vi');

    if (abs >= 1_000_000_000) {
      const v = abs / 1_000_000_000;
      const suffix = isVi ? ' Tỷ' : 'B';
      return sign + fmt(v) + suffix + (isVi ? '' : ' ' + symbol);
    }
    if (abs >= 1_000_000) {
      const v = abs / 1_000_000;
      const suffix = isVi ? ' Triệu' : 'M';
      return sign + fmt(v) + suffix + (isVi ? '' : ' ' + symbol);
    }
    if (!isVi && abs >= 1_000) {
      return sign + fmt(abs / 1_000) + 'K ' + symbol;
    }

    return formatCurrencyByBusinessProfile(num);
  }
}
