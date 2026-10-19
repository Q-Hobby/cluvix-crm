import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
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

import { AutoComplete } from 'primeng/autocomplete';
import { AppointmentTypeCacheService } from './appointment-type-cache.service';
import { AppointmentTypeModel } from '@/core/models/config/appointment/appointment-type.model';
import { TranslateService } from '@ngx-translate/core';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-appointment-type-form-control',
  standalone: true,
  imports: [FormsModule, AutoComplete],
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => AppointmentTypeFormControl),
      multi: true,
    },
  ],
  template: `
    <p-autocomplete
      [(ngModel)]="selectedItem"
      [suggestions]="filteredItems()"
      (completeMethod)="filterItems($event)"
      (onSelect)="onSelect($event)"
      (onClear)="onClear()"
      (onBlur)="onBlur()"
      [dropdown]="true"
      optionLabel="name"
      [showClear]="true"
      appendTo="body"
      [emptyMessage]="emptyMessage"
      class="w-full"
    />
  `,
})
export class AppointmentTypeFormControl implements ControlValueAccessor, OnInit, OnChanges {
  protected emptyMessage = '';
  private translate = inject(TranslateService);
  private cdr = inject(ChangeDetectorRef);
  @Input() activeOnly: boolean = false;
  @Output() valueChange = new EventEmitter<number | null>();
  selectedItem: AppointmentTypeModel | null = null;
  filteredItems = signal<AppointmentTypeModel[]>([]);
  private cache = inject(AppointmentTypeCacheService);
  private pendingValue: number | null = null;

  ngOnChanges(changes: SimpleChanges) {
    if (changes['activeOnly'] && !changes['activeOnly'].firstChange) {
      this.selectedItem = null;
      this.onChange(null);
    }
  }

  async ngOnInit() {
    this.emptyMessage = this.translate.instant('common.label.no_results');
    await this.cache.loadAppointmentTypes();
    if (this.pendingValue !== null && this.selectedItem === null) {
      this.selectedItem = this.cache.getById(this.pendingValue) || null;
      this.cdr.markForCheck();
    }
  }

  filterItems(event: any) {
    const query = (event.query || '').toLowerCase();
    let items = this.getFilteredItems();

    if (!query) {
      // Đảm bảo selected item luôn có trong danh sách khi không có query
      if (this.selectedItem && !items.find((item) => item.id === this.selectedItem!.id)) {
        items = [this.selectedItem, ...items];
      }
      this.filteredItems.set(items.slice(0, 100)); // Giới hạn 100 items khi dropdown
      return;
    }

    const filtered = items.filter((item) => item.name && item.name.toLowerCase().includes(query));

    // Đảm bảo selected item luôn có trong danh sách khi có query
    if (
      this.selectedItem &&
      this.selectedItem.name && this.selectedItem.name.toLowerCase().includes(query) &&
      !filtered.find((item) => item.id === this.selectedItem!.id)
    ) {
      filtered.unshift(this.selectedItem);
    }

    this.filteredItems.set(filtered);
  }

  onSelect(event: any) {
    const value = event.value?.id || null;
    this.onChange(value);
    this.onTouched();
    this.valueChange.emit(value);
  }

  onClear() {
    this.selectedItem = null;
    this.onChange(null);
    this.onTouched();
    this.valueChange.emit(null);
  }

  onBlur() {
    if (typeof this.selectedItem === 'string' || (this.selectedItem && !this.selectedItem.id)) {
      this.selectedItem = null;
      this.onChange(null);
      this.onTouched();
    }
  }

  writeValue(id: number | null): void {
    this.pendingValue = id;
    this.selectedItem = id ? this.cache.getById(id) || null : null;
  }

  registerOnChange(fn: (value: number | null) => void): void {
    this.onChange = fn;
  }

  registerOnTouched(fn: () => void): void {
    this.onTouched = fn;
  }

  private onChange = (value: number | null) => {};

  private onTouched = () => {};

  private getFilteredItems(): AppointmentTypeModel[] {
    return this.cache.getAll(this.activeOnly);
  }
}
