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
import { BranchCacheService } from './branch-cache.service';
import { calculateScrollHeight } from '../base/scroll-height.util';
import { BranchModel } from '@/core/models/config/system/branch.model';
import { TranslateService } from '@ngx-translate/core';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-branch-multiselect-form-control',
  standalone: true,
  imports: [FormsModule, MultiSelect],
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => BranchMultiSelectFormControl),
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
      (onChange)="onSelectionChange($event)"
      (onClear)="onClear()"
    />
  `,
})
export class BranchMultiSelectFormControl implements ControlValueAccessor, OnInit, OnChanges {
  protected emptyMessage = '';
  private translate = inject(TranslateService);
  @Input() activeOnly: boolean = false;
  @Output() valueChange = new EventEmitter<number[] | null>();
  selectedValues: number[] | null = null;
  items = signal<BranchModel[]>([]);
  calculateScrollHeight = calculateScrollHeight;
  private cache = inject(BranchCacheService);

  ngOnChanges(changes: SimpleChanges) {
    if (changes['activeOnly'] && !changes['activeOnly'].firstChange) {
      this.loadItems();
    }
  }

  async ngOnInit() {
    this.emptyMessage = this.translate.instant('common.label.no_results');
    await this.cache.loadBranches();
    this.loadItems();
  }

  onSelectionChange(event: any) {
    const normalizedValue = this.selectedValues ?? [];
    this.onChange(normalizedValue);
    this.onTouched();
    this.valueChange.emit(normalizedValue);
  }

  onClear() {
    this.selectedValues = null;
    this.onChange(null);
    this.onTouched();
    this.valueChange.emit(null);
  }

  writeValue(value: number[] | null): void {
    this.selectedValues = Array.isArray(value) ? value : [];
    if (this.cache.getAllBranchesUnfiltered().length > 0) {
      this.loadItems();
    }
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
    let filteredItems = this.cache.filterBranchesByUser({
      activeOnly: this.activeOnly,
    });

    // Đảm bảo selected items luôn có trong danh sách
    let finalItems = filteredItems;

    if (this.selectedValues && this.selectedValues.length > 0) {
      const selectedItems = this.cache
        .getAllBranchesUnfiltered()
        .filter((item) => this.selectedValues!.includes(item.id!));

      const selectedIds = selectedItems.map((item) => item.id);
      const nonSelectedItems = filteredItems.filter((item) => !selectedIds.includes(item.id));

      finalItems = [...selectedItems, ...nonSelectedItems];
    }

    this.items.set(finalItems);
  }
}
