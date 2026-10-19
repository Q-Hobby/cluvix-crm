import {
  ChangeDetectionStrategy,
  Component,
  EventEmitter,
  forwardRef,
  Input,
  OnChanges,
  OnInit,
  Output,
  signal,
  SimpleChanges,
} from '@angular/core';
import { ControlValueAccessor, FormsModule, NG_VALUE_ACCESSOR } from '@angular/forms';
import { MultiSelect } from 'primeng/multiselect';
import { TranslatePipe } from '@ngx-translate/core';
import { ServiceTypeCacheService } from './service-type-cache.service';
import { calculateScrollHeight } from '@/indexeddb/config/components/base/scroll-height.util';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-service-type-multiselect-form-control',
  standalone: true,
  imports: [FormsModule, MultiSelect, TranslatePipe],
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => ServiceTypeMultiSelectFormControl),
      multi: true,
    },
  ],
  template: `
    <p-multiselect
      [options]="serviceTypeOptions()"
      [(ngModel)]="value"
      (ngModelChange)="onValueChange($event)"
      [disabled]="disabled"
      optionLabel="name"
      optionValue="id"
      [filter]="true"
      appendTo="body"
      [resetFilterOnHide]="true"
      [emptyFilterMessage]="'common.label.no_results' | translate"
      [maxSelectedLabels]="2"
      [virtualScroll]="serviceTypeOptions().length > 20"
      [virtualScrollItemSize]="38"
      [scrollHeight]="calculateScrollHeight(serviceTypeOptions().length)"
      [showClear]="true"
      filterBy="name"
      class="w-full"
      (onClear)="onClear()"
    />
  `,
})
export class ServiceTypeMultiSelectFormControl implements ControlValueAccessor, OnInit, OnChanges {
  @Input() activeOnly: boolean = false;
  @Output() valueChange = new EventEmitter<number[] | null>();

  value: number[] | null = [];
  disabled: boolean = false;
  serviceTypeOptions = signal<{ id: number; name: string }[]>([]);
  protected readonly calculateScrollHeight = calculateScrollHeight;

  constructor(private serviceTypeCache: ServiceTypeCacheService) {}

  async ngOnInit() {
    await this.serviceTypeCache.ensureServiceTypesLoaded();
    this.loadServiceTypes();
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

  onValueChange(value: number[]) {
    this.value = Array.isArray(value) ? value : [];
    this.onChange(value);
    this.onTouched();
  }

  writeValue(value: number[]): void {
    this.value = Array.isArray(value) ? value : [];
    // Reload service types để đảm bảo selected service types có trong danh sách
    if (this.serviceTypeCache.getAllServiceTypes().length > 0) {
      this.loadServiceTypes();
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

  setDisabledState(isDisabled: boolean): void {
    this.disabled = isDisabled;
  }

  private onChange = (value: number[]) => {};

  private onTouched = () => {};

  private loadServiceTypes() {
    const filtered = this.serviceTypeCache.filterServiceTypes({
      activeOnly: this.activeOnly,
      includeServiceTypeIds: this.value || [],
    });

    // Đảm bảo selected service types luôn có trong danh sách
    let finalServiceTypes = filtered;

    if (this.value && this.value.length > 0) {
      // Tìm các selected service types không có trong filtered list
      const allServiceTypes = this.serviceTypeCache.getAllServiceTypes();
      const selectedServiceTypes = allServiceTypes.filter((serviceType) =>
        this.value!.includes(serviceType.id!),
      );

      // Thêm selected service types vào đầu danh sách
      const selectedIds = selectedServiceTypes.map((serviceType) => serviceType.id);
      const nonSelectedServiceTypes = filtered.filter(
        (serviceType) => !selectedIds.includes(serviceType.id),
      );

      finalServiceTypes = [...selectedServiceTypes, ...nonSelectedServiceTypes];
    }

    this.serviceTypeOptions.set(finalServiceTypes.map((s) => ({ id: s.id!, name: s.name! })));
  }
}
