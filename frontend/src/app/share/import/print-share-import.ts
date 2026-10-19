import { CommonModule } from '@angular/common';
import { TranslatePipe } from '@ngx-translate/core';
import { TableModule } from 'primeng/table';

import { CurrencyFormatPipe } from '@/share/pipes/currency-format/currency-format.pipe';
import { CustomDatePipe } from '@/share/pipes/custom-date/custom-date.pipe';
import { NumberFormatPipe } from '@/share/pipes/number-format/number-format.pipe';
import { PhoneFormatPipe } from '@/share/pipes/phone-format/phone-format.pipe';
import { SafeHtmlPipe } from '@/share/pipes/safehtml/safehtml.pipe';

export const PrintShareImport = [
  CommonModule,
  PhoneFormatPipe,
  TableModule,
  CurrencyFormatPipe,
  CustomDatePipe,
  TranslatePipe,
  NumberFormatPipe,
  SafeHtmlPipe,
];
