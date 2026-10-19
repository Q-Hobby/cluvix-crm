import { CommonModule } from '@angular/common';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { TranslatePipe } from '@ngx-translate/core';
import { AutoCompleteModule } from 'primeng/autocomplete';
import {ButtonModule } from 'primeng/button';
import { ButtonGroupModule } from 'primeng/buttongroup';
import { CascadeSelect } from 'primeng/cascadeselect';
import { CheckboxModule } from 'primeng/checkbox';
import { Chip } from 'primeng/chip';
import { ColorPicker } from 'primeng/colorpicker';
import { DatePicker } from 'primeng/datepicker';
import { DialogModule } from 'primeng/dialog';
import { Divider } from 'primeng/divider';
import { Editor } from 'primeng/editor';
import { FileUploadModule } from 'primeng/fileupload';
import { FloatLabel } from 'primeng/floatlabel';
import { Fluid } from 'primeng/fluid';
import { InputGroupModule } from 'primeng/inputgroup';
import { InputGroupAddonModule } from 'primeng/inputgroupaddon';
import { InputMask } from 'primeng/inputmask';
import { InputNumberModule } from 'primeng/inputnumber';
import { InputTextModule } from 'primeng/inputtext';
import { KeyFilterModule } from 'primeng/keyfilter';
import { MultiSelect } from 'primeng/multiselect';
import { Panel } from 'primeng/panel';
import { PasswordModule } from 'primeng/password';
import { Popover } from 'primeng/popover';
import { ProgressSpinnerModule } from 'primeng/progressspinner';
import { RadioButtonModule } from 'primeng/radiobutton';
import { Ripple } from 'primeng/ripple';
import { Select } from 'primeng/select';
import { SelectButton } from 'primeng/selectbutton';
import { Skeleton } from 'primeng/skeleton';
import { Slider } from 'primeng/slider';
import { TableModule } from 'primeng/table';
import { TabsModule } from 'primeng/tabs';
import { Tag } from 'primeng/tag';
import { Textarea } from 'primeng/textarea';
import { ToggleButtonModule } from 'primeng/togglebutton';
import { ToggleSwitchModule } from 'primeng/toggleswitch';
import { TooltipModule } from 'primeng/tooltip';

import { CustomerName } from '@/share/components/customer-name';
import { CustomerPhone } from '@/share/components/customer-phone';
import { DatePickerSingleFooter } from '@/share/components/date-picker-single-footer';
import { DialogFooter } from '@/share/components/dialog-footer';
import { EmptyMessage } from '@/share/components/empty-message/empty-message';
import { CopyTextDirective } from '@/share/directive/copy-text.directive';
import { HasPermDirective } from '@/share/directives/has-perm.directive';
import { AvatarWordPipe } from '@/share/pipes/avatar-word/avatar-word.pipe';
import { CurrencyFormatPipe } from '@/share/pipes/currency-format/currency-format.pipe';
import { CustomDatePipe } from '@/share/pipes/custom-date/custom-date.pipe';
import { IsIncludePipe } from '@/share/pipes/is-include.pipe';
import { IsNegativePipe } from '@/share/pipes/is-negative.pipe';
import { NumberFormatPipe } from '@/share/pipes/number-format/number-format.pipe';
import {
  DepartmentUserMultiselect,
  UserAvatar,
  UserDisplay,
  UserFormControl,
  UserMultiSelectFormControl,
  UserName,
} from '@/indexeddb/config/components/user';
import { AppointmentTypeFormControl } from '@/indexeddb/config/components/appointment-type';
import { DepartmentUserFormControl } from '@/indexeddb/config/components/user/department';
import {
  BranchFormControl,
  BranchMultiSelectFormControl,
} from '@/indexeddb/config/components/branch';
import { UnitFormControl } from '@/indexeddb/config/components/unit';
import { ServiceTypeFormControl } from '@/indexeddb/config/components/service-type';
import { LocationCountryFormControl } from '@/indexeddb/config/components/location-country';
import { LocationProvinceFormControl } from '@/indexeddb/config/components/location-province';
import { AppointmentIsAcceptedAutocompleteFormControl } from '@/indexeddb/config/components/appointment-is-accepted';
import { SourceAutocompleteFormControl } from '@/indexeddb/config/components/source';
import { ServicesAutocompleteFormControl } from '@/indexeddb/config/components/services';
import { Badge } from 'primeng/badge';

export const FormShareImports = [
  CommonModule,
  FormsModule,
  InputNumberModule,
  PasswordModule,
  FileUploadModule,
  RouterLink,
  DialogModule,
  AvatarWordPipe,
  CustomDatePipe,
  DatePickerSingleFooter,
  CascadeSelect,
  Slider,
  Fluid,
  Tag,
  Ripple,
  ButtonModule,
  ToggleButtonModule,
  ProgressSpinnerModule,
  ReactiveFormsModule,
  InputGroupModule,
  InputGroupAddonModule,
  InputTextModule,
  TooltipModule,
  KeyFilterModule,
  AutoCompleteModule,
  RadioButtonModule,
  CheckboxModule,
  TranslatePipe,
  Chip,
  Editor,
  EmptyMessage,
  TabsModule,
  Select,
  SelectButton,
  Skeleton,
  DatePicker,
  FloatLabel,
  NumberFormatPipe,
  CurrencyFormatPipe,
  IsIncludePipe,
  Divider,
  MultiSelect,
  TableModule,
  Fluid,
  Textarea,
  Popover,
  IsNegativePipe,
  ToggleSwitchModule,
  Panel,
  ColorPicker,
  InputMask,
  ButtonGroupModule,
  AutoCompleteModule,
  CopyTextDirective,
  CustomerName,
  CustomerPhone,
  DialogFooter,
  Badge,
  // IndexedDB Components
  AppointmentTypeFormControl,
  UserFormControl,
  DepartmentUserFormControl,
  DepartmentUserMultiselect,
  UserAvatar,
  UserName,
  UserDisplay,
  BranchFormControl,
  UserMultiSelectFormControl,
  BranchMultiSelectFormControl,
  UnitFormControl,
  ServiceTypeFormControl,
  LocationCountryFormControl,
  LocationProvinceFormControl,
  AppointmentIsAcceptedAutocompleteFormControl,
  SourceAutocompleteFormControl,
  ServicesAutocompleteFormControl,
  HasPermDirective,
];
