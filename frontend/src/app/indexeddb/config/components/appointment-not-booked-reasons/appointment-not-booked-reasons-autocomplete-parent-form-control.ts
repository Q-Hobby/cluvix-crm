import { ChangeDetectionStrategy, Component, forwardRef, inject, OnInit, signal } from '@angular/core';
import { ControlValueAccessor, FormsModule, NG_VALUE_ACCESSOR } from '@angular/forms';

import { AutoComplete } from 'primeng/autocomplete';
import { AppointmentNotBookedReasonsCacheService } from './appointment-not-booked-reasons-cache.service';
import { AppointmentNotBookedReasonsModel } from '@/core/models/config/appointment/appointment-not-booked-reasons.model';
import { TranslateService } from '@ngx-translate/core';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-appointment-not-booked-reasons-autocomplete-parent-form-control',
  standalone: true,
  imports: [FormsModule, AutoComplete],
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => AppointmentNotBookedReasonsAutocompleteParentFormControl),
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
export class AppointmentNotBookedReasonsAutocompleteParentFormControl
  implements ControlValueAccessor, OnInit
{
  selectedItem: AppointmentNotBookedReasonsModel | null = null;
  filteredItems = signal<AppointmentNotBookedReasonsModel[]>([]);
  private cache = inject(AppointmentNotBookedReasonsCacheService);
  private cachedParentItems: AppointmentNotBookedReasonsModel[] | null = null;
  private cachedParentItemsVersion = -1;

  protected emptyMessage = '';
  private translate = inject(TranslateService);
  async ngOnInit() {
    this.emptyMessage = this.translate.instant('common.label.no_results');
    await this.cache.ensureReasonsLoaded();
  }

  filterItems(event: any) {
    const query = event.query?.toLowerCase() || '';
    const items = this.getFilteredItems();

    if (!query) {
      this.filteredItems.set(items.slice(0, 100));
      return;
    }

    const filtered = items.filter((item) => item.name && item.name.toLowerCase().includes(query));
    this.filteredItems.set(filtered.slice(0, 200));
  }

  onSelect(event: any) {
    const value = event.value?.id || null;
    this.onChange(value);
    this.onTouched();
  }

  onClear() {
    this.selectedItem = null;
    this.onChange(null);
    this.onTouched();
  }

  onBlur() {
    if (typeof this.selectedItem === 'string' || (this.selectedItem && !this.selectedItem.id)) {
      this.selectedItem = null;
    }
  }

  writeValue(id: number | null): void {
    if (id) {
      this.selectedItem = this.cache.getById(id) || null;
    } else {
      this.selectedItem = null;
    }

    // Reset cache và trigger filterItems để hiển thị selected item
    this.cachedParentItems = null;
    if (this.selectedItem) {
      this.filterItems({ query: '' });
    }
  }

  registerOnChange(fn: (value: number | null) => void): void {
    this.onChange = fn;
  }

  registerOnTouched(fn: () => void): void {
    this.onTouched = fn;
  }

  private onChange = (value: number | null) => {};

  private onTouched = () => {};

  private getFilteredItems(): AppointmentNotBookedReasonsModel[] {
    // Cache parent items để tránh filter lại nhiều lần
    if (!this.cachedParentItems || this.cachedParentItemsVersion !== this.cache.dataVersion) {
      const allItems = this.cache.getAll();
      this.cachedParentItems = allItems.filter((item) => !item.parent_id || item.parent_id === 0);
      this.cachedParentItemsVersion = this.cache.dataVersion;
    }

    let parentItems = [...this.cachedParentItems];

    // Đảm bảo selected item ở đầu danh sách
    if (this.selectedItem && this.selectedItem.id) {
      const selectedIndex = parentItems.findIndex((item) => item.id === this.selectedItem!.id);
      if (selectedIndex > 0) {
        parentItems.splice(selectedIndex, 1);
        parentItems.unshift(this.selectedItem);
      } else if (selectedIndex === -1) {
        // Selected item không phải parent, thêm vào đầu
        parentItems.unshift(this.selectedItem);
      }
    }

    return parentItems;
  }
}
