import { Pipe, PipeTransform } from '@angular/core';
import { authBusinessProfile } from '@/core/services/auth/auth.state';

@Pipe({
  name: 'currencyFormat',
  standalone: true,
})
export class CurrencyFormatPipe implements PipeTransform {
  transform(value: any): string {
    if (value === null || value === undefined || value === '-' || isNaN(value)) {
      return '-';
    }
    return formatCurrencyByBusinessProfile(Number(value));
  }
}

/**
 * Format tiền theo hồ sơ doanh nghiệp (thay `CompanyHelperService.formatCurrency`, story-43 — 1
 * business profile DUY NHẤT cho toàn DB, không còn theo company nào).
 * Ví dụ: formatCurrencyByBusinessProfile(1500000) → "1.500.000 ₫" (mặc định vi-VN).
 */
export function formatCurrencyByBusinessProfile(amount: number): string {
  if (amount == null) return '';
  const profile = authBusinessProfile();
  const locale = profile?.currency_locale || 'vi-VN';
  const symbol = profile?.currency_symbol || '₫';
  const currency = profile?.currency || 'VND';

  try {
    return new Intl.NumberFormat(locale, {
      style: 'currency',
      currency,
      minimumFractionDigits: 0,
      maximumFractionDigits: 0,
    }).format(amount);
  } catch {
    // Fallback nếu currency code không hợp lệ
    const formatted = new Intl.NumberFormat(locale, {
      minimumFractionDigits: 0,
      maximumFractionDigits: 0,
    }).format(amount);
    return locale.startsWith('vi') || locale.startsWith('de') || locale.startsWith('fr')
      ? `${formatted} ${symbol}`
      : `${symbol} ${formatted}`;
  }
}
