import { ChangeDetectionStrategy, Component, forwardRef, inject, OnInit, signal } from '@angular/core';
import { ControlValueAccessor, FormsModule, NG_VALUE_ACCESSOR } from '@angular/forms';

import { MultiSelect } from 'primeng/multiselect';
import { calculateScrollHeight } from '@/indexeddb/config/components/base/scroll-height.util';
import { DepartmentCacheService } from '@/indexeddb/config/components/hr-department/department-cache.service';
import { HrDepartmentModel } from '@/core/models/config/hr/hr-department.model';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'department-multiselect-from-control',
  standalone: true,
  imports: [FormsModule, MultiSelect],
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => DepartementMultiselectFromControl),
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
export class DepartementMultiselectFromControl implements ControlValueAccessor, OnInit {
  selectedIds: number[] | null = [];
  items = signal<HrDepartmentModel[]>([]);
  protected readonly calculateScrollHeight = calculateScrollHeight;
  private cache = inject(DepartmentCacheService);

  async ngOnInit() {
    await this.cache.ensureDepartmentLoaded();
    this.loadItems();
  }

  onSelectionChange(event: any) {
    this.onChange(this.selectedIds ?? []);
    this.onTouched();
  }

  writeValue(value: number[] | null): void {
    this.selectedIds = Array.isArray(value) ? value : [];
    // Luôn reload items để đảm bảo selected items có trong danh sách — guard theo
    // cache.getAll().length trước đây có thể no-op khi cache đang tạm rỗng do invalidation,
    // khiến items() không bao giờ được đồng bộ lại.
    this.loadItems();
  }

  onClear() {
    this.selectedIds = [];
    this.onChange([]);
    this.onTouched();
    this.loadItems();
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

    if (this.selectedIds && this.selectedIds.length > 0) {
      // Tìm các selected items không có trong allItems
      const selectedItems = allItems.filter((item) => this.selectedIds!.includes(item.id!));

      // Thêm selected items vào đầu danh sách
      const selectedIds = selectedItems.map((item) => item.id);
      const nonSelectedItems = allItems.filter((item) => !selectedIds.includes(item.id));

      finalItems = [...selectedItems, ...nonSelectedItems];
    }

    this.items.set(finalItems);
  }
}
