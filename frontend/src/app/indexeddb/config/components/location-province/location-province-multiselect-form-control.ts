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
import { LocationProvinceCacheService } from './location-province-cache.service';
import { calculateScrollHeight } from '@/indexeddb/config/components/base/scroll-height.util';
import { TranslateService } from '@ngx-translate/core';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-location-province-multiselect-form-control',
  standalone: true,
  imports: [FormsModule, MultiSelect],
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => LocationProvinceMultiSelectFormControl),
      multi: true,
    },
  ],
  template: `
    <p-multiselect
      [options]="provinceOptions()"
      [(ngModel)]="value"
      (ngModelChange)="onValueChange($event)"
      optionLabel="label"
      optionValue="value"
      [filter]="true"
      appendTo="body"
      [resetFilterOnHide]="true"
      [emptyFilterMessage]="emptyMessage"
      [maxSelectedLabels]="2"
      [virtualScroll]="provinceOptions().length > 20"
      [virtualScrollItemSize]="38"
      [scrollHeight]="calculateScrollHeight(provinceOptions().length)"
      [showClear]="true"
      filterBy="label"
      [class]="class"
      (onClear)="onClear()"
    />
  `,
})
export class LocationProvinceMultiSelectFormControl
  implements ControlValueAccessor, OnInit, OnChanges
{
  @Input() countryId?: number;
  @Input() class?: string;
  @Output() valueChange = new EventEmitter<number[] | null>();

  value: number[] | null = [];
  provinceOptions = signal<{ value: number; label: string }[]>([]);
  protected readonly calculateScrollHeight = calculateScrollHeight;
  protected emptyMessage = '';
  private translate = inject(TranslateService);

  constructor(private provinceCache: LocationProvinceCacheService) {}

  async ngOnInit() {
    this.emptyMessage = this.translate.instant('common.label.no_results');
    await this.provinceCache.ensureLocationProvincesLoaded();
    this.loadProvinces();
  }

  ngOnChanges(changes: SimpleChanges) {
    if (changes['countryId'] && !changes['countryId'].firstChange) {
      this.loadProvinces();
    }
  }

  onValueChange(value: number[]) {
    this.value = Array.isArray(value) ? value : [];
    this.onChange(value);
    this.onTouched();
    this.valueChange.emit(value);
  }

  writeValue(value: number[]): void {
    this.value = Array.isArray(value) ? value : [];
    // Reload provinces để đảm bảo selected provinces có trong danh sách
    if (this.provinceCache.getAllLocationProvinces().length > 0) {
      this.loadProvinces();
    }
  }

  onClear() {
    this.value = null;
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

  private loadProvinces() {
    const filtered = this.provinceCache.filterLocationProvinces({
      activeOnly: true,
      countryId: this.countryId,
      includeLocationProvinceIds: this.value || [],
    });

    // Đảm bảo selected provinces luôn có trong danh sách
    let finalProvinces = filtered;

    if (this.value && this.value.length > 0) {
      // Tìm các selected provinces không có trong filtered list
      const allProvinces = this.provinceCache.getAllLocationProvinces();
      const selectedProvinces = allProvinces.filter((province) => this.value!.includes(province.id!));

      // Thêm selected provinces vào đầu danh sách
      const selectedIds = selectedProvinces.map((province) => province.id);
      const nonSelectedProvinces = filtered.filter(
        (province) => !selectedIds.includes(province.id),
      );

      finalProvinces = [...selectedProvinces, ...nonSelectedProvinces];
    }

    // Sort và giới hạn số provinces để tăng performance
    const sorted = finalProvinces.sort((a, b) => (a.SortOrder ?? 0) - (b.SortOrder ?? 0));

    this.provinceOptions.set(sorted.map((p) => ({ value: p.id!, label: p.name! })));
  }
}
