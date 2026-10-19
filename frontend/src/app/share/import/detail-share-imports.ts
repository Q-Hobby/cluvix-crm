import { CommonModule } from '@angular/common';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
import { BadgeModule } from 'primeng/badge';
import { ButtonModule } from 'primeng/button';
import { ButtonGroup } from 'primeng/buttongroup';
import { CardModule } from 'primeng/card';
import { ChipModule } from 'primeng/chip';
import { DialogModule } from 'primeng/dialog';
import { DividerModule } from 'primeng/divider';
import { InputText } from 'primeng/inputtext';
import { ProgressSpinner } from 'primeng/progressspinner';
import { RippleModule } from 'primeng/ripple';
import { SelectButtonModule } from 'primeng/selectbutton';
import { SkeletonModule } from 'primeng/skeleton';
import { TableModule } from 'primeng/table';
import { TabsModule } from 'primeng/tabs';
import { Tag, TagModule } from 'primeng/tag';
import { TooltipModule } from 'primeng/tooltip';

import { CustomerName } from '@/share/components/customer-name';
import { CustomerPhone } from '@/share/components/customer-phone';
import { EmptyMessage } from '@/share/components/empty-message/empty-message';
import { CopyTextDirective } from '@/share/directive/copy-text.directive';
import { HasPermDirective } from '@/share/directives/has-perm.directive';
import { PswpLightboxDirective } from '@/share/directive/pswp-lightbox.directive';
import { AvatarWordPipe } from '@/share/pipes/avatar-word/avatar-word.pipe';
import { CurrencyFormatPipe } from '@/share/pipes/currency-format/currency-format.pipe';
import { CustomDatePipe } from '@/share/pipes/custom-date/custom-date.pipe';
import { EllipsisTextPipe } from '@/share/pipes/ellipsis-text/ellipsis-text.pipe';
import { SafeHtmlPipe } from '@/share/pipes/safehtml/safehtml.pipe';
import { SanitizeQuillPipe } from '@/share/pipes/sanitize-quill.pipe';
import { ToArrayPipe } from '@/share/pipes/to-array.pipe';

export const DetailShareImports = [
  CommonModule,
  DividerModule,
  TagModule,
  BadgeModule,
  AvatarWordPipe,
  ButtonModule,
  TooltipModule,
  RippleModule,
  SkeletonModule,
  CurrencyFormatPipe,
  CustomDatePipe,
  SelectButtonModule,
  ReactiveFormsModule,
  ChipModule,
  CardModule,
  TranslateModule,
  EllipsisTextPipe,
  TabsModule,
  ProgressSpinner,
  Tag,
  TableModule,
  ToArrayPipe,
  EmptyMessage,
  SafeHtmlPipe,
  ButtonGroup,
  DialogModule,
  CopyTextDirective,
  HasPermDirective,
  CustomerName,
  CustomerPhone,
  SanitizeQuillPipe,
  PswpLightboxDirective,
  InputText,
  FormsModule,
];
