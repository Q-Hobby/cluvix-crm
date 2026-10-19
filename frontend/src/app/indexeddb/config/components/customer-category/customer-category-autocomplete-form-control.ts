import {
  ChangeDetectionStrategy,
  Component,
  EventEmitter,
  forwardRef,
  inject,
  OnInit,
  Output,
  signal,
} from '@angular/core';
import { ControlValueAccessor, FormsModule, NG_VALUE_ACCESSOR } from '@angular/forms';

import { AutoComplete } from 'primeng/autocomplete';
import { CustomerCategoryCacheService } from './customer-category-cache.service';
import { calculateScrollHeight } from '@/indexeddb/config/components/base/scroll-height.util';
import { CustomerCategoryModel } from '@/core/models/config/marketing/customer-category.model';
import { TranslateService } from '@ngx-translate/core';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-customer-category-autocomplete-form-control',
  standalone: true,
  imports: [FormsModule, AutoComplete],
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => CustomerCategoryAutocompleteFormControl),
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
export class CustomerCategoryAutocompleteFormControl
  implements ControlValueAccessor, OnInit
{
  protected emptyMessage = '';
  private translate = inject(TranslateService);
  @Output() valueChange = new EventEmitter<number | null>();
  selectedItem: CustomerCategoryModel | null = null;
  filteredItems = signal<CustomerCategoryModel[]>([]);
  protected readonly calculateScrollHeight = calculateScrollHeight;
  private cache = inject(CustomerCategoryCacheService);

  async ngOnInit() {
    this.emptyMessage = this.translate.instant('common.label.no_results');
    await this.cache.ensureCustomerCategorysLoaded();
  }

  filterItems(event: any) {
    const query = (event.query || '').toLowerCase();
    let items = this.getFilteredItemsWithSelected();

    if (!query) {
      this.filteredItems.set(items.slice(0, 100)); // Giới hạn 100 items khi dropdown
      return;
    }

    const filtered = items.filter((item) => item.name && item.name.toLowerCase().includes(query));
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
    }
  }

  writeValue(id: number | null): void {
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

  private getFilteredItemsWithSelected(): CustomerCategoryModel[] {
    let items = this.getFilteredItems();

    // Đảm bảo selected campaign luôn có trong danh sách
    let finalItems = items;

    if (this.selectedItem) {
      // Thêm selected campaign vào đầu danh sách
      const nonSelectedItems = items.filter((item) => item.id !== this.selectedItem!.id);
      finalItems = [this.selectedItem, ...nonSelectedItems];
    }

    return finalItems;
  }

  private getFilteredItems(): CustomerCategoryModel[] {
    return this.cache.getAll();
  }
}
