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
  ViewChild,
} from '@angular/core';
import { ControlValueAccessor, FormsModule, NG_VALUE_ACCESSOR } from '@angular/forms';

import { MultiSelect } from 'primeng/multiselect';
import { AppointmentNotBookedReasonsCacheService } from './appointment-not-booked-reasons-cache.service';
import { calculateScrollHeight } from '../base/scroll-height.util';
import { TranslateService } from '@ngx-translate/core';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-appointment-not-booked-reasons-multiselect-form-control',
  standalone: true,
  imports: [FormsModule, MultiSelect],
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => AppointmentNotBookedReasonsMultiselectFormControl),
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
      [style]="style"
      [size]="size"
      class="w-full"
      (onChange)="onSelectionChange($event)"
      (onClear)="onClear()"
    />
  `,
})
export class AppointmentNotBookedReasonsMultiselectFormControl
  implements ControlValueAccessor, OnInit, OnChanges
{
  @ViewChild(MultiSelect) multiSelect!: MultiSelect;
  protected emptyMessage = '';
  private translate = inject(TranslateService);
  @Input() style: any = {};
  @Input() size: 'small' | 'large' | undefined;
  @Input() value: number[] | null = null;
  @Input() itemType: 'parent' | 'children' = 'children'; // 'parent' = chỉ lấy parent items, 'children' = chỉ lấy children items
  @Output() valueChange = new EventEmitter<number[] | null>();
  selectedValues: number[] | null = null;
  items = signal<any[]>([]);
  calculateScrollHeight = calculateScrollHeight;
  private cache = inject(AppointmentNotBookedReasonsCacheService);
  private cdr = inject(ChangeDetectorRef);
  private isInitialized = false;

  async ngOnInit() {
    this.emptyMessage = this.translate.instant('common.label.no_results');
    await this.cache.ensureReasonsLoaded();
    this.loadItems();

    requestAnimationFrame(() => {
      this.selectedValues = this.value;
      this.isInitialized = true;
      this.cdr.markForCheck();
    });
  }

  ngOnChanges(changes: SimpleChanges) {
    if (changes['value'] && this.items().length > 0) {
      this.selectedValues = this.value;
      this.cdr.markForCheck();
    }

    if (changes['itemType'] && !changes['itemType'].firstChange) {
      this.loadItems();
    }
  }

  onSelectionChange(event: any) {
    const value = event.value || null;

    this.selectedValues = Array.isArray(value) ? value : [];
    this.onChange(value);
    this.onTouched();

    if (this.isInitialized) {
      this.valueChange.emit(value);
    }
  }

  writeValue(value: number[] | null): void {
    this.selectedValues = Array.isArray(value) ? value : [];
    if (this.cache.getAll().length > 0) {
      this.loadItems();
    }
  }

  onClear() {
    this.selectedValues = null;
    this.onChange(null);
    this.onTouched();
    this.valueChange.emit(null);
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
    const allItems = this.cache.getAll();

    // Chọn parents hoặc children dựa trên itemType property
    const filteredItems =
      this.itemType === 'parent'
        ? allItems.filter((item) => !item.parent_id || item.parent_id === 0) // Parents (parent_id == null)
        : allItems.filter((item) => item.parent_id && item.parent_id > 0); // Children (parent_id != null)

    // Đảm bảo selected items ở đầu
    let finalItems = filteredItems;

    if (this.selectedValues && this.selectedValues.length > 0) {
      const selectedItems = filteredItems.filter((item) => this.selectedValues!.includes(item.id!));
      const nonSelectedItems = filteredItems.filter(
        (item) => !this.selectedValues!.includes(item.id!),
      );
      finalItems = [...selectedItems, ...nonSelectedItems];
    }

    this.items.set(finalItems);
  }
}
