import { CommonModule } from '@angular/common';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { TranslatePipe } from '@ngx-translate/core';
import { QrCodeComponent } from 'ng-qrcode';
import { Avatar } from 'primeng/avatar';
import { BadgeModule } from 'primeng/badge';
import { ButtonModule } from 'primeng/button';
import { ButtonGroupModule } from 'primeng/buttongroup';
import { CheckboxModule } from 'primeng/checkbox';
import { ChipModule } from 'primeng/chip';
import { ColorPicker } from 'primeng/colorpicker';
import { ConfirmDialogModule } from 'primeng/confirmdialog';
import { DataView } from 'primeng/dataview';
import { DatePicker } from 'primeng/datepicker';
import { DialogModule } from 'primeng/dialog';
import { DividerModule } from 'primeng/divider';
import { FileUpload } from 'primeng/fileupload';
import { FloatLabel } from 'primeng/floatlabel';
import { IconField } from 'primeng/iconfield';
import { InputGroupModule } from 'primeng/inputgroup';
import { InputGroupAddonModule } from 'primeng/inputgroupaddon';
import { InputIcon } from 'primeng/inputicon';
import { InputTextModule } from 'primeng/inputtext';
import { Menu } from 'primeng/menu';
import { MultiSelectModule } from 'primeng/multiselect';
import { PaginatorModule } from 'primeng/paginator';
import { Panel } from 'primeng/panel';
import { Popover } from 'primeng/popover';
import { ProgressBar } from 'primeng/progressbar';
import { ProgressSpinner } from 'primeng/progressspinner';
import { RadioButton } from 'primeng/radiobutton';
import { RippleModule } from 'primeng/ripple';
import { Select } from 'primeng/select';
import { SkeletonModule } from 'primeng/skeleton';
import { StyleClass } from 'primeng/styleclass';
import { TableModule } from 'primeng/table';
import { TabsModule } from 'primeng/tabs';
import { TagModule } from 'primeng/tag';
import { ToastModule } from 'primeng/toast';
import { ToggleSwitchModule } from 'primeng/toggleswitch';
import { ToolbarModule } from 'primeng/toolbar';
import { TooltipModule } from 'primeng/tooltip';

import { AvatarLazy } from '@/share/components/app-avatar-lazy';
import { AppColumnToggle } from '@/share/components/app-column-toggle';
import { ChipLazyViewport } from '@/share/components/chip-lazy-viewport';
import { CustomerName } from '@/share/components/customer-name';
import { CustomerPhone } from '@/share/components/customer-phone';
import { CustomerTimelineButton } from '@/share/components/customer-timeline-button';
import { DatePickerFooter } from '@/share/components/date-picker-footer';
import { DatePickerLargeFooter } from '@/share/components/date-picker-large-footer';
import { EmptyMessage } from '@/share/components/empty-message/empty-message';
import { ImageLightbox } from '@/share/components/image-lightbox';
import { CopyTextDirective } from '@/share/directive/copy-text.directive';
import { PswpLightboxDirective } from '@/share/directive/pswp-lightbox.directive';
import { HasPermDirective } from '@/share/directives/has-perm.directive';
import { CurrencyFormatPipe } from '@/share/pipes/currency-format/currency-format.pipe';
import { CustomDatePipe } from '@/share/pipes/custom-date/custom-date.pipe';
import { IsIncludePipe } from '@/share/pipes/is-include.pipe';
import { IsNegativePipe } from '@/share/pipes/is-negative.pipe';
import { JoinPipe } from '@/share/pipes/join.pipe';
import { NumberFormatPipe } from '@/share/pipes/number-format/number-format.pipe';
import { LazyImgPipe } from '@/share/pipes/lazy-img.pipe';
import { SafeHtmlPipe } from '@/share/pipes/safehtml/safehtml.pipe';
import { SanitizeQuillPipe } from '@/share/pipes/sanitize-quill.pipe';
import { ToArrayPipe } from '@/share/pipes/to-array.pipe';
import { FormatMonthPipe } from '@/share/pipes/format-month.pipe';

// IndexedDB Components
import { AppointmentTypeMultiSelectFormControl } from '@/indexeddb/config/components/appointment-type';
import {
  DepartmentUserMultiselect,
  UserMultiSelectFormControl,
} from '@/indexeddb/config/components/user';
import { BranchMultiSelectFormControl } from '@/indexeddb/config/components/branch';
import { ServiceTypeMultiSelectFormControl } from '@/indexeddb/config/components/service-type';
import {
  AppointmentIsAcceptedAutocompleteFormControl,
  AppointmentIsAcceptedMultiselectFormControl,
} from '@/indexeddb/config/components/appointment-is-accepted';
import { SourceMultiselectFormControl } from '@/indexeddb/config/components/source';
import {
  ServicesAutocompleteFormControl,
  ServicesMultiselectFormControl,
} from '@/indexeddb/config/components/services';

export const ListShareImports = [
  CommonModule,
  PaginatorModule,
  ToolbarModule,
  CustomerTimelineButton,
  ToastModule,
  ConfirmDialogModule,
  ColorPicker,
  DialogModule,
  QrCodeComponent,
  ProgressSpinner,
  DatePickerLargeFooter,
  DataView,
  TagModule,
  InputGroupModule,
  InputGroupAddonModule,
  StyleClass,
  Avatar,
  Select,
  RadioButton,
  NumberFormatPipe,
  BadgeModule,
  TableModule,
  ButtonModule,
  TooltipModule,
  RippleModule,
  ReactiveFormsModule,
  JoinPipe,
  IsIncludePipe,
  MultiSelectModule,
  DividerModule,
  InputTextModule,
  RouterLink,
  ChipModule,
  CurrencyFormatPipe,
  CustomDatePipe,
  ToArrayPipe,
  SkeletonModule,
  EmptyMessage,
  CheckboxModule,
  FormsModule,
  TranslatePipe,
  IsNegativePipe,
  Popover,
  DatePicker,
  FloatLabel,
  Panel,
  IconField,
  InputIcon,
  ButtonGroupModule,
  SafeHtmlPipe,
  Menu,
  FileUpload,
  TabsModule,
  ProgressBar,
  DatePickerFooter,
  ToggleSwitchModule,
  CopyTextDirective,
  CustomerName,
  CustomerPhone,
  PswpLightboxDirective,
  SanitizeQuillPipe,
  LazyImgPipe,
  AppColumnToggle,
  ImageLightbox,
  AvatarLazy,
  ChipLazyViewport,
  // IndexedDB Components
  AppointmentTypeMultiSelectFormControl,
  UserMultiSelectFormControl,
  DepartmentUserMultiselect,
  BranchMultiSelectFormControl,
  ServiceTypeMultiSelectFormControl,
  AppointmentIsAcceptedAutocompleteFormControl,
  SourceMultiselectFormControl,
  AppointmentIsAcceptedMultiselectFormControl,
  ServicesAutocompleteFormControl,
  ServicesMultiselectFormControl,
  HasPermDirective,
  FormatMonthPipe,
];
