import {
  ChangeDetectionStrategy,
  Component,
  EventEmitter,
  forwardRef,
  inject,
  Input,
  OnInit,
  Output,
  signal,
} from '@angular/core';
import { ControlValueAccessor, FormsModule, NG_VALUE_ACCESSOR } from '@angular/forms';

import { AutoComplete } from 'primeng/autocomplete';
import { CustomerRankServicesCacheService } from '@/indexeddb/config/components/customer-rank-services/customer-rank-services-cache.service';
import { CustomerRankServicesModel } from '@/core/models/config/customer/customer-rank-services.model';
import { TranslateService } from '@ngx-translate/core';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'customer-rank-services-autocomplete-form-control',
  standalone: true,
  imports: [FormsModule, AutoComplete],
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => CustomerRankServicesAutocompleteFormControl),
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
      [size]="size"
      [autofocus]="false"
      [style]="style"
      optionLabel="name"
      [showClear]="true"
      appendTo="body"
      [emptyMessage]="emptyMessage"
      class="w-full"
    />
  `,
})
export class CustomerRankServicesAutocompleteFormControl implements ControlValueAccessor, OnInit {
  protected emptyMessage = '';
  private translate = inject(TranslateService);
  @Input() size: 'small' | 'large' | undefined;
  @Output() valueChange = new EventEmitter<number | null>();
  @Input() style: any = {};
  selectedItem: CustomerRankServicesModel | null = null;
  filteredItems = signal<CustomerRankServicesModel[]>([]);
  private cache = inject(CustomerRankServicesCacheService);

  async ngOnInit() {
    this.emptyMessage = this.translate.instant('common.label.no_results');
    await this.cache.ensureCustomerRankServicesLoaded();
  }

  filterItems(event: any) {
    const query = event.query || '';
    let items = this.getFilteredItems();

    if (!query) {
      this.filteredItems.set(items.slice(0, 100)); // Giới hạn 100 items khi dropdown
      return;
    }

    const filtered = items.filter((item) =>
      String(item.rank_id || '')
        .toLowerCase()
        .includes(query.toLowerCase()),
    );
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

  private getFilteredItems(): CustomerRankServicesModel[] {
    let allItems = this.cache.getAll();

    // Đảm bảo selected item luôn có trong danh sách
    let finalItems = allItems;

    if (this.selectedItem) {
      // Thêm selected item vào đầu danh sách
      const nonSelectedItems = allItems.filter((item) => item.id !== this.selectedItem!.id);
      finalItems = [this.selectedItem, ...nonSelectedItems];
    }

    return finalItems;
  }
}
