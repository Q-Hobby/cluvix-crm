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

import { AutoComplete } from 'primeng/autocomplete';
import { LocationCountryCacheService } from './location-country-cache.service';
import { calculateScrollHeight } from '@/indexeddb/config/components/base/scroll-height.util';
import { LocationCountryModel } from '@/core/models/config/location/location-country.model';
import { TranslateService } from '@ngx-translate/core';
import { normalizeSearchText } from '@/share/utils/text-normalize.util';
import { MAX_SEARCH_SUGGESTIONS } from '@/share/utils/search-limit.const';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-location-country-form-control',
  standalone: true,
  imports: [FormsModule, AutoComplete],
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => LocationCountryFormControl),
      multi: true,
    },
  ],
  template: `
    <p-autocomplete
      [(ngModel)]="selectedCountry"
      [suggestions]="filteredCountries"
      (completeMethod)="filterItems($event)"
      (onSelect)="onCountrySelect($event)"
      (onClear)="onClear()"
      (onBlur)="onBlur()"
      [dropdown]="true"
      [showClear]="true"
      [appendTo]="appendTo"
      [emptyMessage]="emptyFilterMessage"
      [minQueryLength]="0"
      optionLabel="CommonName"
      [virtualScroll]="filteredCountries.length > 20"
      [virtualScrollItemSize]="40"
      [scrollHeight]="calculateScrollHeight(filteredCountries.length, 40)"
      class="w-full"
    >
      <ng-template let-country #item>
        <div class="flex items-center gap-2">
          <img
            [src]="'https://flagcdn.com/w20/' + country.CountryCode.toLowerCase() + '.png'"
            [alt]="country.CommonName"
            class="w-5 h-auto"
          />
          <span>{{ country.CommonName }}</span>
        </div>
      </ng-template>
    </p-autocomplete>
  `,
})
export class LocationCountryFormControl implements ControlValueAccessor, OnInit, OnChanges {
  @Input() activeOnly: boolean = false;
  @Input() appendTo: any = 'body';
  @Input() emptyFilterMessage: string = '';
  @Output() valueChange = new EventEmitter<number | null>();

  value: number | null = null;
  filteredCountries: LocationCountryModel[] = [];
  selectedCountry: any = null;
  locationCountries = signal<LocationCountryModel[]>([]);
  protected readonly calculateScrollHeight = calculateScrollHeight;
  private translate = inject(TranslateService);

  constructor(private locationCountryCache: LocationCountryCacheService) {}

  async ngOnInit() {
    if (!this.emptyFilterMessage) {
      this.emptyFilterMessage = this.translate.instant('common.label.no_results');
    }
    await this.locationCountryCache.loadLocationCountries();
    this.loadLocationCountries();
    this.updateDisplayValue();
  }

  ngOnChanges(changes: SimpleChanges) {
    if (changes['activeOnly'] && !changes['activeOnly'].firstChange) {
      this.loadLocationCountries();
    }
  }

  filterItems(event: any) {
    // Lọc trực tiếp trên locationCountries() (KHÔNG qua getSearchable() — getSearchable() chỉ có
    // record active, còn locationCountries() có thể chứa thêm 1 record inactive đã chọn sẵn qua
    // includeLocationCountryId ở loadLocationCountries(); dùng getSearchable() làm allowlist sẽ làm
    // mất record đó khỏi kết quả search). 245 record — normalize tại chỗ vẫn rẻ, không cần memoize
    // riêng cho nhánh này.
    const source = this.locationCountries();
    if (!event.query) {
      this.filteredCountries = source.slice(0, MAX_SEARCH_SUGGESTIONS);
      return;
    }
    const query = normalizeSearchText(event.query);
    const filtered: LocationCountryModel[] = [];
    for (const item of source) {
      if (normalizeSearchText(item.CommonName).includes(query)) {
        filtered.push(item);
        if (filtered.length >= MAX_SEARCH_SUGGESTIONS) break;
      }
    }
    this.filteredCountries = filtered;
  }

  onCountrySelect(event: any) {
    const selected = event.value || event;
    if (selected && selected.id) {
      this.value = selected.id;
      this.selectedCountry = selected;
      this.onChange(selected.id);
      this.onTouched();
      this.valueChange.emit(selected.id);
    }
  }

  onClear() {
    this.value = null;
    this.selectedCountry = null;
    this.onChange(null);
    this.onTouched();
    this.valueChange.emit(null);
  }

  onBlur() {
    this.updateDisplayValue();
  }

  writeValue(value: number | null): void {
    this.value = value;
    // Reload countries để đảm bảo selected country có trong danh sách
    if (this.locationCountryCache.getAllLocationCountries().length > 0) {
      this.loadLocationCountries();
    }
    this.updateDisplayValue();
  }

  registerOnChange(fn: (value: number | null) => void): void {
    this.onChange = fn;
  }

  registerOnTouched(fn: () => void): void {
    this.onTouched = fn;
  }

  private onChange = (value: number | null) => {};

  private onTouched = () => {};

  private loadLocationCountries() {
    const filtered = this.locationCountryCache.filterLocationCountries({
      activeOnly: true,
      includeLocationCountryId: this.value || undefined,
    });

    // Đảm bảo selected country luôn có trong danh sách
    let finalCountries = filtered;

    if (this.value) {
      const allCountries = this.locationCountryCache.getAllLocationCountries();
      const selectedCountry = allCountries.find((country) => country.id === this.value);

      if (selectedCountry) {
        // Thêm selected country vào đầu danh sách
        const nonSelectedCountries = filtered.filter((country) => country.id !== this.value);
        finalCountries = [selectedCountry, ...nonSelectedCountries];
      }
    }

    // Sort và giới hạn số countries để tăng performance
    const sorted = finalCountries.sort((a, b) => (a.SortOrder ?? 0) - (b.SortOrder ?? 0));

    this.locationCountries.set(sorted);
  }

  private updateDisplayValue() {
    const selected = this.locationCountries().find((lc) => lc.id === this.value);
    this.selectedCountry = selected || null;
  }
}
