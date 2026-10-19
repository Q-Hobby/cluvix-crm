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

import { Select } from 'primeng/select';
import { LocationProvinceCacheService } from './location-province-cache.service';
import { TranslateService } from '@ngx-translate/core';
import { calculateScrollHeight } from '@/indexeddb/config/components/base/scroll-height.util';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-location-province-form-control',
  standalone: true,
  imports: [FormsModule, Select],
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => LocationProvinceFormControl),
      multi: true,
    },
  ],
  template: `
    <p-select
      [ngModel]="value"
      (ngModelChange)="onModelChange($event)"
      [options]="provinceOptions()"
      optionLabel="label"
      optionValue="value"
      [filter]="true"
      [showClear]="true"
      appendTo="body"
      [emptyFilterMessage]="emptyMessage"
      filterBy="label"
      [virtualScroll]="provinceOptions().length > 20"
      [virtualScrollItemSize]="38"
      [scrollHeight]="calculateScrollHeight(provinceOptions().length)"
      class="w-full"
    />
  `,
})
export class LocationProvinceFormControl implements ControlValueAccessor, OnInit, OnChanges {
  @Input() countryId?: number;
  @Output() valueChange = new EventEmitter<number | null>();

  value: number | null = null;
  provinceOptions = signal<{ value: number; label: string }[]>([]);
  protected emptyMessage = '';
  protected readonly calculateScrollHeight = calculateScrollHeight;
  private initialized = false;
  private translate = inject(TranslateService);

  constructor(private provinceCache: LocationProvinceCacheService) {}

  async ngOnInit() {
    this.emptyMessage = this.translate.instant('common.label.no_results');
    await this.provinceCache.ensureLocationProvincesLoaded();
    this.initialized = true;
    this.loadProvinces();
  }

  ngOnChanges(changes: SimpleChanges) {
    if (changes['countryId'] && this.initialized) {
      this.loadProvinces();
    }
  }

  onModelChange(newValue: number | null) {
    this.value = newValue;
    this.onChange(this.value);
    this.onTouched();
    this.valueChange.emit(this.value);
  }

  writeValue(value: number | null): void {
    this.value = value;
    if (this.initialized) {
      this.loadProvinces();
    }
  }

  registerOnChange(fn: (value: number | null) => void): void {
    this.onChange = fn;
  }

  registerOnTouched(fn: () => void): void {
    this.onTouched = fn;
  }

  setDisabledState?(isDisabled: boolean): void {
    // Handle disabled state if needed
  }

  private onChange = (value: number | null) => {};

  private onTouched = () => {};

  private loadProvinces() {
    const filtered = this.provinceCache.filterLocationProvinces({
      activeOnly: true,
      countryId: this.countryId,
      includeLocationProvinceId: this.value || undefined,
    });

    // Đảm bảo selected province luôn có trong danh sách
    let finalProvinces = filtered;

    if (this.value) {
      const allProvinces = this.provinceCache.getAllLocationProvinces();
      const selectedProvince = allProvinces.find((province) => province.id === this.value);

      if (selectedProvince) {
        // Thêm selected province vào đầu danh sách
        const nonSelectedProvinces = filtered.filter((province) => province.id !== this.value);
        finalProvinces = [selectedProvince, ...nonSelectedProvinces];
      }
    }

    // Sort và giới hạn số provinces để tăng performance
    const sorted = finalProvinces.sort((a, b) => (a.SortOrder ?? 0) - (b.SortOrder ?? 0));

    this.provinceOptions.set(sorted.map((p) => ({ value: p.id!, label: p.name! })));
  }
}
