import {
  ChangeDetectionStrategy,
  Component,
  EventEmitter,
  forwardRef,
  inject,
  Input,
  OnChanges,
  OnInit,
  Output,
  signal,
  SimpleChanges,
} from '@angular/core';
import { ControlValueAccessor, FormsModule, NG_VALUE_ACCESSOR } from '@angular/forms';

import { MultiSelect } from 'primeng/multiselect';
import { AppointmentTypeCacheService } from './appointment-type-cache.service';
import { calculateScrollHeight } from '@/indexeddb/config/components/base/scroll-height.util';
import { AppointmentTypeModel } from '@/core/models/config/appointment/appointment-type.model';
import { TranslateService } from '@ngx-translate/core';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-appointment-type-multiselect-form-control',
  standalone: true,
  imports: [FormsModule, MultiSelect],
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => AppointmentTypeMultiSelectFormControl),
      multi: true,
    },
  ],
  template: `
    <p-multiselect
      [(ngModel)]="selectedValues"
      [options]="items()"
      optionLabel="name"
      optionValue="id"
      [showClear]="true"
      [filter]="true"
      filterBy="name"
      [emptyMessage]="emptyMessage"
      appendTo="body"
      [maxSelectedLabels]="2"
      [virtualScroll]="items().length > 20"
      [virtualScrollItemSize]="38"
      [scrollHeight]="calculateScrollHeight(items().length)"
      class="w-full"
      [style]="style"
      (onChange)="onSelectionChange($event)"
      (onClear)="onClear()"
    />
  `,
})
export class AppointmentTypeMultiSelectFormControl
  implements ControlValueAccessor, OnInit, OnChanges
{
  protected emptyMessage = '';
  private translate = inject(TranslateService);
  @Input() style: any = {};
  @Input() activeOnly: boolean = false;
  @Output() valueChange = new EventEmitter<number[] | null>();
  selectedValues: number[] | null = null;
  items = signal<AppointmentTypeModel[]>([]);
  protected readonly calculateScrollHeight = calculateScrollHeight;
  private cache = inject(AppointmentTypeCacheService);

  ngOnChanges(changes: SimpleChanges) {
    if (changes['activeOnly'] && !changes['activeOnly'].firstChange) {
      this.loadItems();
    }
  }

  async ngOnInit() {
    this.emptyMessage = this.translate.instant('common.label.no_results');
    await this.cache.loadAppointmentTypes();
    this.loadItems();
  }

  onSelectionChange(event: any) {
    const value = event.value || null;
    this.onChange(value);
    this.onTouched();
    this.valueChange.emit(value);
  }

  writeValue(value: number[] | null): void {
    this.selectedValues = Array.isArray(value) ? value : [];
    // Reload items để đảm bảo selected items có trong danh sách
    if (this.cache.getAll().length > 0) {
      this.loadItems();
    }
  }

  onClear() {
    this.selectedValues = null;
    this.onChange(null);
    this.onTouched();
    this.valueChange.emit(null);
  }

  registerOnChange(fn: (value: number[] | null) => void): void {
    this.onChange = fn;
  }

  registerOnTouched(fn: () => void): void {
    this.onTouched = fn;
  }

  private onChange = (value: number[] | null) => {};

  private onTouched = () => {};

  private loadItems(): void {
    let filteredItems = this.cache.getAll();

    if (this.activeOnly) {
      filteredItems = filteredItems.filter((item) => item.status === 1);
    }

    // Đảm bảo selected items luôn có trong danh sách
    let finalItems = filteredItems;

    if (this.selectedValues && this.selectedValues.length > 0) {
      // Tìm các selected items không có trong filtered list
      const selectedItems = this.cache
        .getAll()
        .filter((item) => this.selectedValues!.includes(item.id!));

      // Thêm selected items vào đầu danh sách
      const selectedIds = selectedItems.map((item) => item.id);
      const nonSelectedItems = filteredItems.filter((item) => !selectedIds.includes(item.id));

      finalItems = [...selectedItems, ...nonSelectedItems];
    }

    this.items.set(finalItems);
  }
}
