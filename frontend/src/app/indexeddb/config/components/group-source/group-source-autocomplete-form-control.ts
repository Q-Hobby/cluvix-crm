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
  TemplateRef,
} from '@angular/core';
import { ControlValueAccessor, FormsModule, NG_VALUE_ACCESSOR } from '@angular/forms';
import { CommonModule } from '@angular/common';
import { AutoComplete } from 'primeng/autocomplete';
import { GroupSourceCacheService } from './group-source-cache.service';
import { calculateScrollHeight } from '@/indexeddb/config/components/base/scroll-height.util';
import { GroupSourceModel } from '@/core/models/config/marketing/group-source.model';
import { TranslateService } from '@ngx-translate/core';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-group-source-autocomplete-form-control',
  standalone: true,
  imports: [CommonModule, FormsModule, AutoComplete],
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => GroupSourceAutocompleteFormControl),
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
    >
      <ng-template #footer>
        <ng-container *ngTemplateOutlet="footerTemplate"></ng-container>
      </ng-template>
    </p-autocomplete>
  `,
})
export class GroupSourceAutocompleteFormControl implements ControlValueAccessor, OnInit {
  protected emptyMessage = '';
  private translate = inject(TranslateService);
  @Output() valueChange = new EventEmitter<number | null>();
  selectedItem: GroupSourceModel | null = null;
  filteredItems = signal<GroupSourceModel[]>([]);
  @Input() footerTemplate?: TemplateRef<any>;
  protected readonly calculateScrollHeight = calculateScrollHeight;
  private cache = inject(GroupSourceCacheService);

  async ngOnInit() {
    this.emptyMessage = this.translate.instant('common.label.no_results');
    await this.cache.ensureGroupSourceLoaded();
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

  private getFilteredItemsWithSelected(): GroupSourceModel[] {
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

  private getFilteredItems(): GroupSourceModel[] {
    return this.cache.getAll();
  }
}
