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
import { TranslateService } from '@ngx-translate/core';
import { SourceCacheService } from './source-cache.service';
import { calculateScrollHeight } from '@/indexeddb/config/components/base/scroll-height.util';
import { SourceModel } from '@/core/models/config/marketing/source.model';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-source-multiselect-form-control',
  standalone: true,
  imports: [FormsModule, MultiSelect],
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => SourceMultiselectFormControl),
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
export class SourceMultiselectFormControl implements ControlValueAccessor, OnInit, OnChanges {
  @Input() includeNull: boolean = false;
  @Output() valueChange = new EventEmitter<number[] | null>();
  selectedIds: number[] | null = [];
  items = signal<SourceModel[]>([]);
  protected readonly calculateScrollHeight = calculateScrollHeight;
  private cache = inject(SourceCacheService);
  private translate = inject(TranslateService);

  async ngOnInit() {
    await this.cache.ensureSourcesLoaded();
    this.loadItems();
  }

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['includeNull']) {
      this.loadItems();
    }
  }

  onSelectionChange(event: any) {
    this.onChange(this.selectedIds ?? []);
    this.onTouched();
  }

  writeValue(value: number[] | null): void {
    this.selectedIds = Array.isArray(value) ? value : [];
    // Reload items để đảm bảo selected items có trong danh sách
    if (this.cache.getAll().length > 0) {
      this.loadItems();
    }
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

    // Đảm bảo selected items luôn có trong danh sách
    let finalItems = items;

    if (this.selectedIds && this.selectedIds.length > 0) {
      // Tìm các selected items không có trong filtered list
      const allItems = this.cache.getAll();
      const selectedItems = allItems.filter((item) => this.selectedIds!.includes(item.id!));

      // Thêm selected items vào đầu danh sách
      const selectedIds = selectedItems.map((item) => item.id);
      const nonSelectedItems = items.filter((item) => !selectedIds.includes(item.id));

      finalItems = [...selectedItems, ...nonSelectedItems];
    }

    if (this.includeNull) {
      const nullOption = { id: 0, name: this.translate.instant('common.label.unassigned') } as SourceModel;
      finalItems = [nullOption, ...finalItems.filter((i) => i.id !== 0)];
    }

    this.items.set(finalItems);
  }

  private getFilteredItems(): SourceModel[] {
    return this.cache.getAll();
  }
}
