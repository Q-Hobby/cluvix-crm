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
import { BranchCacheService } from './branch-cache.service';
import { calculateScrollHeight } from '@/indexeddb/config/components/base/scroll-height.util';
import { BranchModel } from '@/core/models/config/system/branch.model';
import { TranslateService } from '@ngx-translate/core';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-branch-form-control',
  standalone: true,
  imports: [FormsModule, AutoComplete],
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => BranchFormControl),
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
      [dropdown]="true"
      (onBlur)="onBlur()"
      optionLabel="name"
      [inputId]="inputId"
      [showClear]="true"
      appendTo="body"
      [emptyMessage]="emptyMessage"
      class="w-full"
    />
  `,
})
export class BranchFormControl implements ControlValueAccessor, OnInit, OnChanges {
  protected emptyMessage = '';
  private translate = inject(TranslateService);
  @Input() selectedValue: number | null = null;
  @Input() activeOnly: boolean = false;
  @Input() statusOpenAppointment?: boolean;
  // Gắn <label for> của p-floatlabel bao ngoài vào input thật bên trong autocomplete (a11y).
  @Input() inputId?: string;
  @Output() valueChange = new EventEmitter<number | null>();
  selectedItem: BranchModel | null = null;
  filteredItems = signal<BranchModel[]>([]);
  protected readonly calculateScrollHeight = calculateScrollHeight;
  private cache = inject(BranchCacheService);
  private cdr = inject(ChangeDetectorRef);
  private currentValue: number | null = null;

  ngOnChanges(changes: SimpleChanges) {
    // Các input lọc khác giữ nguyên hành vi cũ: đổi là reset thẳng.
    if (
      (changes['activeOnly'] && !changes['activeOnly'].firstChange) ||
      (changes['statusOpenAppointment'] && !changes['statusOpenAppointment'].firstChange)
    ) {
      this.selectedItem = null;
      this.onChange(null);
      return;
    }
  }

  async ngOnInit() {
    this.emptyMessage = this.translate.instant('common.label.no_results');
    await this.cache.loadBranches();
  }

  filterItems(event: any) {
    const query = (event?.query || '').toLowerCase();
    let items = this.getFilteredItems();

    if (!query) {
      if (this.selectedItem && !items.find((item) => item.id === this.selectedItem!.id)) {
        items = [this.selectedItem, ...items];
      }
      this.filteredItems.set(items.slice(0, 100)); // Giới hạn 100 items khi dropdown
      return;
    }

    const filtered = items.filter((item) => item.name && item.name.toLowerCase().includes(query));

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
    this.currentValue = value;
    this.onChange(value);
    this.onTouched();
    this.valueChange.emit(value);
  }

  onClear() {
    this.selectedItem = null;
    this.currentValue = null;
    this.onChange(null);
    this.onTouched();
    this.valueChange.emit(null);
  }

  writeValue(id: number | null): void {
    this.currentValue = id;
    if (!id) {
      this.selectedItem = null;
      this.cdr.markForCheck();
      return;
    }
    const found = this.cache.getAllBranchesUnfiltered().find((b) => b.id === id) || null;
    if (found) {
      this.selectedItem = found;
      this.cdr.markForCheck();
    } else {
      this.cache.loadBranches().then(() => {
        this.selectedItem = this.cache.getAllBranchesUnfiltered().find((b) => b.id === id) || null;
        this.cdr.markForCheck();
      });
    }
  }

  registerOnChange(fn: (value: number | null) => void): void {
    this.onChange = fn;
  }

  registerOnTouched(fn: () => void): void {
    this.onTouched = fn;
  }

  onBlur() {
    if (typeof this.selectedItem === 'string' || (this.selectedItem && !this.selectedItem.id)) {
      this.selectedItem = this.currentValue
        ? this.cache.getAllBranchesUnfiltered().find((b) => b.id === this.currentValue) || null
        : null;
    }
  }

  private onChange = (value: number | null) => {};

  private onTouched = () => {};

  private getFilteredItems(): BranchModel[] {
    let items = this.cache.filterBranchesByUser({
      activeOnly: this.activeOnly,
    });

    if (this.statusOpenAppointment !== undefined) {
      const filterValue = this.statusOpenAppointment ? 1 : 0;
      items = items.filter((item) => item.statusOpenAppointment === filterValue);
    }

    return items;
  }
}
