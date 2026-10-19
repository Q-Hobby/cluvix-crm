import {
  ChangeDetectionStrategy,
  Component,
  EventEmitter,
  forwardRef,
  inject,
  DestroyRef,
  Input,
  OnChanges,
  OnInit,
  Output,
  signal,
  SimpleChanges,
} from '@angular/core';
import { ControlValueAccessor, FormsModule, NG_VALUE_ACCESSOR } from '@angular/forms';

import { Subject } from 'rxjs';
import { debounceTime } from 'rxjs/operators';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { MultiSelect } from 'primeng/multiselect';
import { ServicesCacheService } from './services-cache.service';
import { ServiceModel } from '@/core/models/config/clinic/service.model';
import { calculateScrollHeight } from '@/indexeddb/config/components/base/scroll-height.util';
import { TranslateService } from '@ngx-translate/core';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-services-multiselect-form-control',
  standalone: true,
  imports: [FormsModule, MultiSelect],
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => ServicesMultiselectFormControl),
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
      filterBy="name,code"
      [emptyMessage]="emptyMessage"
      appendTo="body"
      [maxSelectedLabels]="2"
      [virtualScroll]="items().length > 20"
      [virtualScrollItemSize]="38"
      [scrollHeight]="calculateScrollHeight(items().length)"
      [disabled]="disabled"
      class="w-full"
      (onChange)="onSelectionChange($event)"
      (onClear)="onClear()"
    />
  `,
})
export class ServicesMultiselectFormControl implements ControlValueAccessor, OnInit, OnChanges {
  protected emptyMessage = '';
  private translate = inject(TranslateService);
  @Input() serviceTypeId: number | null = null;
  @Input() status: number | null = null;
  @Input() disabled: boolean = false;
  @Input() ignoreTypeFilter: boolean = false;
  @Output() valueChange = new EventEmitter<number[] | null>();
  selectedValues: number[] | null = null;
  items = signal<ServiceModel[]>([]);
  protected readonly calculateScrollHeight = calculateScrollHeight;
  private cache = inject(ServicesCacheService);
  private filterTrigger$ = new Subject<void>();
  private destroyRef = inject(DestroyRef);
  private cachedFilteredItems: ServiceModel[] = [];
  private lastFilterKey = '';

  ngOnChanges(changes: SimpleChanges) {
    if (
      (changes['serviceTypeId'] && !changes['serviceTypeId'].firstChange) ||
      (changes['status'] && !changes['status'].firstChange)
    ) {
      // Clear cache khi filter parameters thay đổi
      this.lastFilterKey = '';
      this.cachedFilteredItems = [];
      this.filterTrigger$.next();
    }
  }

  async ngOnInit() {
    this.emptyMessage = this.translate.instant('common.label.no_results');
    this.filterTrigger$.pipe(debounceTime(50), takeUntilDestroyed(this.destroyRef)).subscribe(() => this.loadItems());
    await this.cache.ensureServicesLoaded();
    this.loadItems();
  }

  onSelectionChange(event: any) {
    const value = event.value || null;
    this.onChange(value);
    this.onTouched();
    this.valueChange.emit(value);
  }

  writeValue(value: number[] | null): void {
    this.selectedValues = Array.isArray(value) ? value : [];
    // Reload items để đảm bảo selected items có trong danh sách
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

  setDisabledState(isDisabled: boolean): void {
    this.disabled = isDisabled;
  }

  private onChange = (value: number[] | null) => {};

  private onTouched = () => {};

  private loadItems(): void {
    const filterKey = `${this.serviceTypeId || ''}_${this.status ?? ''}_v${this.cache.dataVersion}`;

    let filteredItems: ServiceModel[];
    if (this.lastFilterKey !== filterKey) {
      const base = this.ignoreTypeFilter ? this.cache.getAllActiveIgnoreTypeFilter() : this.cache.getAll();
      filteredItems = base.filter((item) => {
        if (this.serviceTypeId && item.service_type_id !== this.serviceTypeId) return false;
        if (this.status !== null && this.status !== undefined && item.status !== this.status) return false;
        return true;
      });
      this.cachedFilteredItems = filteredItems;
      this.lastFilterKey = filterKey;
    } else {
      filteredItems = this.cachedFilteredItems;
    }

    // Đảm bảo selected items luôn có trong danh sách
    if (this.selectedValues && this.selectedValues.length > 0) {
      const selectedIds = new Set(this.selectedValues);
      const selectedItems = this.cache
        .getAllServicesUnfiltered()
        .filter((item) => selectedIds.has(item.id!));

      const nonSelectedItems = filteredItems.filter((item) => !selectedIds.has(item.id!));
      this.items.set([...selectedItems, ...nonSelectedItems]);
    } else {
      this.items.set(filteredItems);
    }
  }
}
