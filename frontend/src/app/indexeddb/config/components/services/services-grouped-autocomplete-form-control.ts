import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  computed,
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
import { TranslatePipe } from '@ngx-translate/core';
import { AutoComplete } from 'primeng/autocomplete';
import { ServicesCacheService } from './services-cache.service';
import { ServiceModel } from '@/core/models/config/clinic/service.model';
import { TranslateService } from '@ngx-translate/core';

interface GroupedService {
  label: string;
  labelKey?: string;
  items: ServiceModel[];
}

interface CustomerServiceWithProgress {
  id: number;
  name: string;
}

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-services-grouped-autocomplete-form-control',
  standalone: true,
  imports: [FormsModule, AutoComplete, TranslatePipe],
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => ServicesGroupedAutocompleteFormControl),
      multi: true,
    },
  ],
  template: `
    <p-autocomplete
      [(ngModel)]="selectedItem"
      [suggestions]="filteredItems()"
      (completeMethod)="filterItems($event)"
      (onSelect)="onSelect($event)"
      (onClear)="onClear()"
      (onBlur)="onBlur()"
      [dropdown]="true"
      [disabled]="disabled"
      [group]="true"
      [virtualScroll]="totalFilteredCount() > 20"
      [virtualScrollItemSize]="38"
      optionLabel="name"
      [showClear]="true"
      appendTo="body"
      [emptyMessage]="emptyMessage"
      class="w-full"
    >
      <ng-template let-group #group>
        <div class="px-3 py-2 bg-gray-50 font-semibold text-sm text-gray-700">
          <span>{{ (group.labelKey || group.label) | translate }}</span>
        </div>
      </ng-template>
    </p-autocomplete>
  `,
})
export class ServicesGroupedAutocompleteFormControl implements ControlValueAccessor, OnInit, OnChanges {
  protected emptyMessage = '';
  private translate = inject(TranslateService);
  private cdr = inject(ChangeDetectorRef);
  @Input() serviceTypeId: number | null = null;
  @Input() status: number | null = null;
  @Input() customerServices: CustomerServiceWithProgress[] = [];
  @Output() valueChange = new EventEmitter<number | null>();
  @Output() selectService = new EventEmitter<{ value: any; index: number }>();
  @Input() index?: number;
  @Input() disabled: boolean = false;
  selectedItem: ServiceModel | null = null;
  filteredItems = signal<GroupedService[]>([]);
  totalFilteredCount = computed(() => this.filteredItems().reduce((sum, g) => sum + g.items.length, 0));
  private cache = inject(ServicesCacheService);
  private cachedFilteredItems: ServiceModel[] = [];
  private lastFilterKey = '';
  private pendingId: number | null = null;
  private readonly MAX_RESULTS = 100;

  ngOnChanges(changes: SimpleChanges) {
    const hasRelevantChange =
      (changes['serviceTypeId'] && !changes['serviceTypeId'].firstChange) ||
      (changes['status'] && !changes['status'].firstChange) ||
      (changes['customerServices'] && !changes['customerServices'].firstChange);

    if (hasRelevantChange) {
      this.lastFilterKey = '';
      this.cachedFilteredItems = [];

      if (this.selectedItem) {
        const newFilteredItems = this.getFilteredItems();
        if (!newFilteredItems.find((item) => item?.id === this.selectedItem!.id)) {
          this.selectedItem = null;
          this.onChange(null);
        }
      }
    }
  }

  async ngOnInit() {
    this.emptyMessage = this.translate.instant('common.label.no_results');
    await this.cache.ensureServicesLoaded();
    if (this.pendingId && !this.selectedItem) {
      this.writeValue(this.pendingId);
      this.cdr.markForCheck();
    }
  }

  filterItems(event: any) {
    const query = (event?.query ?? '').toString().trim().toLowerCase();
    this.performFilter(query);
  }

  private performFilter(query: string) {
    let items = this.getFilteredItems();
    const safe = (v: any) => (v == null ? '' : String(v)).toLowerCase();
    const csSet = new Set(this.customerServices.map(s => s.id));

    if (query && query.length >= 3) {
      const keywords = query.split(/\s+/).filter(k => k.length > 0);
      items = items.filter((item) => {
        if (!item) return false;
        const searchText = `${safe(item.name)} ${safe(item.code)}`;
        return keywords.every(keyword => searchText.includes(keyword));
      }).slice(0, this.MAX_RESULTS);
    } else if (query && query.length < 3) {
      const customerItems = items.filter(item => csSet.has(item.id!));
      const otherItems = items.filter(item => !csSet.has(item.id!)).slice(0, 50);
      items = [...customerItems, ...otherItems];
    }
    // No query: show all items — virtualScroll handles rendering

    if (this.selectedItem && !items.find((item) => item?.id === this.selectedItem!.id)) {
      if (!query || query.length >= 3) {
        const keywords = query ? query.split(/\s+/).filter(k => k.length > 0) : [];
        const searchText = `${safe(this.selectedItem.name)} ${safe(this.selectedItem.code)}`;
        if (!query || keywords.every(keyword => searchText.includes(keyword))) {
          items = [this.selectedItem, ...items];
        }
      }
    }

    const grouped = this.groupServices(items);
    this.filteredItems.set(grouped);
  }

  onSelect(event: any) {
    const value = event.value || null;
    if (this.index !== undefined) {
      this.selectService.emit({ value, index: this.index });
    }
    const id = value?.id || null;
    this.valueChange.emit(id);
    this.onChange(id);
    this.onTouched();
  }

  onClear() {
    this.selectedItem = null;
    this.valueChange.emit(null);
    this.onChange(null);
    this.onTouched();
  }

  onBlur() {
    if (this.selectedItem && !this.selectedItem.id) {
      this.selectedItem = null;
      this.onChange(null);
      this.onTouched();
    }
  }

  writeValue(id: number | null): void {
    this.pendingId = id;
    if (!id) {
      this.selectedItem = null;
      return;
    }

    const customerService = this.customerServices.find(s => s.id === id);
    if (customerService) {
      const cachedService = this.cache.getById(id);
      this.selectedItem = cachedService ? {
        ...cachedService,
        name: customerService.name,
      } : null;
    } else {
      this.selectedItem = this.cache.getById(id) || null;
    }
  }

  registerOnChange(fn: (value: number | null) => void): void {
    this.onChange = fn;
  }

  registerOnTouched(fn: () => void): void {
    this.onTouched = fn;
  }

  setDisabledState(isDisabled: boolean): void {
    this.disabled = isDisabled;
  }

  private onChange = (value: number | null) => {};

  private onTouched = () => {};

  private groupServices(items: ServiceModel[]): GroupedService[] {
    if (!this.customerServices || this.customerServices.length === 0) {
      return [{ label: 'Tất cả dịch vụ', labelKey: 'share.services_autocomplete.all', items }];
    }

    const csSet = new Set(this.customerServices.map(s => s.id));
    const customerServiceMap = new Map(this.customerServices.map(s => [s.id, s.name]));
    const customerServicesWithProgress: ServiceModel[] = [];
    const otherServices: ServiceModel[] = [];

    items.forEach((item) => {
      if (csSet.has(item.id!)) {
        customerServicesWithProgress.push({
          ...item,
          name: customerServiceMap.get(item.id!) || item.name,
        });
      } else {
        otherServices.push(item);
      }
    });

    const groups: GroupedService[] = [];

    if (customerServicesWithProgress.length > 0) {
      groups.push({
        label: 'Dịch vụ khách hàng đã đặt',
        labelKey: 'share.services_autocomplete.customer_ordered',
        items: customerServicesWithProgress,
      });
    }

    if (otherServices.length > 0) {
      groups.push({
        label: 'Dịch vụ khác',
        labelKey: 'share.services_autocomplete.other',
        items: otherServices,
      });
    }

    return groups;
  }

  private getFilteredItems(): ServiceModel[] {
    const filterKey = `${this.serviceTypeId || ''}_${this.status ?? ''}_v${this.cache.dataVersion}`;

    if (this.lastFilterKey !== filterKey) {
      const base = this.cache.getAll();
      this.cachedFilteredItems = base.filter((item) => {
        if (this.serviceTypeId && item.service_type_id !== this.serviceTypeId) return false;
        if (this.status !== null && this.status !== undefined && item.status !== this.status) return false;
        return true;
      });
      this.lastFilterKey = filterKey;
    }

    return this.cachedFilteredItems;
  }
}
