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

import { MultiSelect } from 'primeng/multiselect';
import { GroupSourceCacheService } from './group-source-cache.service';
import { calculateScrollHeight } from '@/indexeddb/config/components/base/scroll-height.util';
import { GroupSourceModel } from '@/core/models/config/marketing/group-source.model';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-group-source-multiselect-form-control',
  standalone: true,
  imports: [FormsModule, MultiSelect],
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => GroupSourceMultiselectFormControl),
      multi: true,
    },
  ],
  template: `
    <p-multiselect
      [(ngModel)]="selectedIds"
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
      (onClear)="onClear()"
    />
  `,
})
export class GroupSourceMultiselectFormControl implements ControlValueAccessor, OnInit {
  @Output() valueChange = new EventEmitter<number[] | null>();
  selectedIds: number[] | null = [];
  items = signal<GroupSourceModel[]>([]);
  protected readonly calculateScrollHeight = calculateScrollHeight;
  private cache = inject(GroupSourceCacheService);

  async ngOnInit() {
    await this.cache.ensureGroupSourceLoaded();
    this.loadItems();
  }

  onSelectionChange(event: any) {
    this.onChange(this.selectedIds ?? []);
    this.onTouched();
  }

  writeValue(value: number[] | null): void {
    this.selectedIds = Array.isArray(value) ? value : [];
    // Đảm bảo load items ngay cả khi cache chưa sẵn sàng
    this.loadItems();
  }

  onClear() {
    this.selectedIds = null;
    this.onChange([]);
    this.onTouched();
    this.valueChange.emit(null);
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
    let items = this.getFilteredItems();

    // Đảm bảo selected campaigns luôn có trong danh sách
    let finalItems = items;

    if (this.selectedIds && this.selectedIds.length > 0) {
      // Luôn tìm selected items từ tất cả campaigns
      const allItems = this.cache.getAll();
      const selectedItems = this.selectedIds
        .map((id) => allItems.find((item) => item.id === id))
        .filter((item) => item !== undefined) as GroupSourceModel[];

      // Thêm selected campaigns vào đầu danh sách
      const selectedIds = selectedItems.map((item) => item.id);
      const nonSelectedItems = items.filter((item) => !selectedIds.includes(item.id));

      finalItems = [...selectedItems, ...nonSelectedItems];
    }

    this.items.set(finalItems);
  }

  private getFilteredItems(): GroupSourceModel[] {
    return this.cache.getAll();
  }
}
