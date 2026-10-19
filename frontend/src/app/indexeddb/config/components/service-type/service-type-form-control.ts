import {
  ChangeDetectionStrategy,
  Component,
  forwardRef,
  inject,
  Input,
  OnChanges,
  OnInit,
  signal,
  SimpleChanges,
} from '@angular/core';
import { ControlValueAccessor, FormsModule, NG_VALUE_ACCESSOR } from '@angular/forms';

import { AutoComplete } from 'primeng/autocomplete';
import { ServiceTypeCacheService } from './service-type-cache.service';
import { TranslateService } from '@ngx-translate/core';
import { normalizeSearchText } from '@/share/utils/text-normalize.util';
import { MAX_SEARCH_SUGGESTIONS } from '@/share/utils/search-limit.const';
import { calculateScrollHeight } from '@/indexeddb/config/components/base/scroll-height.util';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-service-type-form-control',
  standalone: true,
  imports: [FormsModule, AutoComplete],
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => ServiceTypeFormControl),
      multi: true,
    },
  ],
  template: `
    <p-autocomplete
      [(ngModel)]="displayValue"
      [suggestions]="filteredNames"
      (completeMethod)="filterItems($event)"
      (onSelect)="onNameSelect($event)"
      (onClear)="onClear()"
      (onBlur)="onBlur()"
      [dropdown]="true"
      [showClear]="true"
      [appendTo]="appendTo"
      [emptyMessage]="emptyFilterMessage"
      [minQueryLength]="0"
      [virtualScroll]="filteredNames.length > 20"
      [virtualScrollItemSize]="38"
      [scrollHeight]="calculateScrollHeight(filteredNames.length)"
      class="w-full"
    />
  `,
})
export class ServiceTypeFormControl implements ControlValueAccessor, OnInit, OnChanges {
  protected readonly calculateScrollHeight = calculateScrollHeight;
  @Input() activeOnly: boolean = false;
  @Input() appendTo: any = 'body';
  @Input() emptyFilterMessage: string = '';

  value: number | null = null;
  filteredNames: string[] = [];
  displayValue: string = '';
  serviceTypes = signal<any[]>([]);
  private translate = inject(TranslateService);

  constructor(private serviceTypeCache: ServiceTypeCacheService) {}

  async ngOnInit() {
    if (!this.emptyFilterMessage) {
      this.emptyFilterMessage = this.translate.instant('common.label.no_results');
    }
    await this.serviceTypeCache.loadServiceTypes();
    this.loadServiceTypes();
    this.updateDisplayValue();
  }

  ngOnChanges(changes: SimpleChanges) {
    const relevantChanges = ['activeOnly'];
    const hasRelevantChange = relevantChanges.some(
      (key) => changes[key] && !changes[key].firstChange,
    );

    if (hasRelevantChange) {
      this.loadServiceTypes();
    }
  }

  filterItems(event: any) {
    if (!event.query) {
      this.filteredNames = this.serviceTypes()
        .slice(0, MAX_SEARCH_SUGGESTIONS)
        .map((st) => st.name!);
      return;
    }
    const query = normalizeSearchText(event.query);
    const filtered: string[] = [];
    for (const st of this.serviceTypes()) {
      if (st.name && normalizeSearchText(st.name).includes(query)) {
        filtered.push(st.name);
        if (filtered.length >= MAX_SEARCH_SUGGESTIONS) break;
      }
    }
    this.filteredNames = filtered;
  }

  onNameSelect(event: any) {
    const name = event.value || event;
    const selected = this.serviceTypes().find((st) => st.name === name);
    if (selected) {
      this.value = selected.id;
      this.displayValue = selected.name;
      this.onChange(selected.id);
      this.onTouched();
    }
  }

  onClear() {
    this.value = null;
    this.displayValue = '';
    this.onChange(null);
    this.onTouched();
  }

  onBlur() {
    this.updateDisplayValue();
  }

  writeValue(value: number | null): void {
    this.value = value;
    // Reload service types để đảm bảo selected service type có trong danh sách
    if (this.serviceTypeCache.getAllServiceTypes().length > 0) {
      this.loadServiceTypes();
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

  private loadServiceTypes() {
    const filtered = this.serviceTypeCache.filterServiceTypes({
      activeOnly: this.activeOnly,
      includeServiceTypeId: this.value || undefined,
    });

    // Đảm bảo selected service type luôn có trong danh sách
    let finalServiceTypes = filtered;

    if (this.value) {
      const allServiceTypes = this.serviceTypeCache.getAllServiceTypes();
      const selectedServiceType = allServiceTypes.find((st) => st.id === this.value);

      if (selectedServiceType) {
        // Thêm selected service type vào đầu danh sách
        const nonSelectedServiceTypes = filtered.filter((st) => st.id !== this.value);
        finalServiceTypes = [selectedServiceType, ...nonSelectedServiceTypes];
      }
    }

    this.serviceTypes.set(finalServiceTypes.filter((st) => st.name));
  }

  private updateDisplayValue() {
    const selected = this.serviceTypes().find((st) => st.id === this.value);
    this.displayValue = selected?.name || '';
  }
}
