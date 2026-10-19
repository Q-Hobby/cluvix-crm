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
import { LocationCountryCacheService } from './location-country-cache.service';
import { calculateScrollHeight } from '@/indexeddb/config/components/base/scroll-height.util';
import { LocationCountryModel } from '@/core/models/config/location/location-country.model';
import { TranslateService } from '@ngx-translate/core';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-location-country-multiselect-form-control',
  standalone: true,
  imports: [FormsModule, MultiSelect],
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => LocationCountryMultiSelectFormControl),
      multi: true,
    },
  ],
  template: `
    <p-multiselect
      [options]="locationCountryOptions()"
      [(ngModel)]="value"
      (ngModelChange)="onValueChange($event)"
      optionLabel="CommonName"
      optionValue="id"
      [filter]="true"
      appendTo="body"
      [resetFilterOnHide]="true"
      [emptyFilterMessage]="emptyMessage"
      [maxSelectedLabels]="2"
      [virtualScroll]="locationCountryOptions().length > 20"
      [virtualScrollItemSize]="38"
      [scrollHeight]="calculateScrollHeight(locationCountryOptions().length)"
      [showClear]="true"
      filterBy="name"
      class="w-full"
      (onClear)="onClear()"
    >
      <ng-template let-country #item>
        <div class="flex items-center gap-2">
          <img
            [src]="'https://flagcdn.com/w20/' + (country.CountryCode || '').toLowerCase() + '.png'"
            [alt]="country.CommonName"
            class="w-5 h-auto"
          />
          <span>{{ country.CommonName }}</span>
        </div>
      </ng-template>
    </p-multiselect>
  `,
})
export class LocationCountryMultiSelectFormControl
  implements ControlValueAccessor, OnInit, OnChanges
{
  @Input() activeOnly: boolean = false;
  @Output() valueChange = new EventEmitter<number[] | null>();

  value: number[] | null = [];
  locationCountryOptions = signal<LocationCountryModel[]>([]);
  protected readonly calculateScrollHeight = calculateScrollHeight;
  protected emptyMessage = '';
  private translate = inject(TranslateService);

  constructor(private locationCountryCache: LocationCountryCacheService) {}

  async ngOnInit() {
    this.emptyMessage = this.translate.instant('common.label.no_results');
    await this.locationCountryCache.ensureLocationCountriesLoaded();
    this.loadLocationCountries();
  }

  ngOnChanges(changes: SimpleChanges) {
    if (changes['activeOnly'] && !changes['activeOnly'].firstChange) {
      this.loadLocationCountries();
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
    // Reload countries để đảm bảo selected countries có trong danh sách
    if (this.locationCountryCache.getAllLocationCountries().length > 0) {
      this.loadLocationCountries();
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

  private loadLocationCountries() {
    const filtered = this.locationCountryCache.filterLocationCountries({
      activeOnly: this.activeOnly,
      includeLocationCountryIds: this.value || [],
    });

    // Đảm bảo selected countries luôn có trong danh sách
    let finalCountries = filtered;

    if (this.value && this.value.length > 0) {
      // Tìm các selected countries không có trong filtered list
      const allCountries = this.locationCountryCache.getAllLocationCountries();
      const selectedCountries = allCountries.filter((country) => this.value!.includes(country.id!));

      // Thêm selected countries vào đầu danh sách
      const selectedIds = selectedCountries.map((country) => country.id);
      const nonSelectedCountries = filtered.filter((country) => !selectedIds.includes(country.id));

      finalCountries = [...selectedCountries, ...nonSelectedCountries];
    }

    // Sort và giới hạn số countries để tăng performance
    const sorted = finalCountries.sort((a, b) => (a.SortOrder ?? 0) - (b.SortOrder ?? 0));

    this.locationCountryOptions.set(sorted);
  }
}
