import { ChangeDetectionStrategy, Component, forwardRef, inject, OnInit, signal } from '@angular/core';
import { ControlValueAccessor, FormsModule, NG_VALUE_ACCESSOR } from '@angular/forms';

import { MultiSelect } from 'primeng/multiselect';
import { AppointmentIsAcceptedCacheService } from './appointment-is-accepted-cache.service';
import { calculateScrollHeight } from '@/indexeddb/config/components/base/scroll-height.util';
import { AppointmentIsAcceptedModel } from '@/core/models/config/appointment/appointment-is-accepted.model';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-appointment-is-accepted-multiselect-form-control',
  standalone: true,
  imports: [FormsModule, MultiSelect],
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => AppointmentIsAcceptedMultiselectFormControl),
      multi: true,
    },
  ],
  template: `
    <p-multiselect
      [(ngModel)]="selectedItems"
      [options]="items()"
      (onChange)="onSelectionChange($event)"
      [filter]="true"
      [showClear]="true"
      optionLabel="name"
      optionValue="id"
      appendTo="body"
      [maxSelectedLabels]="2"
      [virtualScroll]="items().length > 20"
      [virtualScrollItemSize]="38"
      [scrollHeight]="calculateScrollHeight(items().length)"
      class="w-full"
    />
  `,
})
export class AppointmentIsAcceptedMultiselectFormControl implements ControlValueAccessor, OnInit {
  selectedItems: number[] = [];
  items = signal<AppointmentIsAcceptedModel[]>([]);
  protected readonly calculateScrollHeight = calculateScrollHeight;
  private cache = inject(AppointmentIsAcceptedCacheService);

  async ngOnInit() {
    await this.cache.ensureAppointmentIsAcceptedsLoaded();
    this.loadItems();
  }

  onSelectionChange(event: any) {
    this.onChange(this.selectedItems);
    this.onTouched();
  }

  writeValue(value: number[]): void {
    this.selectedItems = value || [];
    // Reload items để đảm bảo selected items có trong danh sách
    if (this.cache.getAll().length > 0) {
      this.loadItems();
    }
  }

  registerOnChange(fn: (value: number[]) => void): void {
    this.onChange = fn;
  }

  registerOnTouched(fn: () => void): void {
    this.onTouched = fn;
  }

  private onChange = (value: number[]) => {};

  private onTouched = () => {};

  private loadItems() {
    let allItems = this.cache.getAll();

    // Đảm bảo selected items luôn có trong danh sách
    let finalItems = allItems;

    if (this.selectedItems && this.selectedItems.length > 0) {
      // Tìm các selected items không có trong allItems
      const selectedItemsData = allItems.filter((item) => this.selectedItems.includes(item.id!));

      // Thêm selected items vào đầu danh sách
      const selectedIds = selectedItemsData.map((item) => item.id);
      const nonSelectedItems = allItems.filter((item) => !selectedIds.includes(item.id));

      finalItems = [...selectedItemsData, ...nonSelectedItems];
    }

    this.items.set(finalItems);
  }
}
