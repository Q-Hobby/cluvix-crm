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
import { UnitCacheService } from './unit-cache.service';
import { calculateScrollHeight } from '@/indexeddb/config/components/base/scroll-height.util';
import { TranslateService } from '@ngx-translate/core';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-unit-multiselect-form-control',
  standalone: true,
  imports: [FormsModule, MultiSelect],
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => UnitMultiSelectFormControl),
      multi: true,
    },
  ],
  template: `
    <p-multiselect
      [options]="unitOptions()"
      [(ngModel)]="value"
      (ngModelChange)="onValueChange($event)"
      optionLabel="name"
      optionValue="id"
      [filter]="true"
      appendTo="body"
      [resetFilterOnHide]="true"
      [showClear]="true"
      [emptyFilterMessage]="emptyMessage"
      [maxSelectedLabels]="2"
      [virtualScroll]="unitOptions().length > 20"
      [virtualScrollItemSize]="38"
      [scrollHeight]="calculateScrollHeight(unitOptions().length)"
      filterBy="name"
      class="w-full"
      (onClear)="onClear()"
    />
  `,
})
export class UnitMultiSelectFormControl implements ControlValueAccessor, OnInit, OnChanges {
  @Input() activeOnly: boolean = false;
  @Input() category?: number;
  @Output() valueChange = new EventEmitter<number[] | null>();

  value: number[] | null = [];
  unitOptions = signal<{ id: number; name: string }[]>([]);
  protected readonly calculateScrollHeight = calculateScrollHeight;
  protected emptyMessage = '';
  private translate = inject(TranslateService);

  constructor(private unitCache: UnitCacheService) {}

  async ngOnInit() {
    this.emptyMessage = this.translate.instant('common.label.no_results');
    await this.unitCache.loadUnits();
    this.loadUnits();
  }

  ngOnChanges(changes: SimpleChanges) {
    const relevantChanges = ['category', 'activeOnly'];
    const hasRelevantChange = relevantChanges.some(
      (key) => changes[key] && !changes[key].firstChange,
    );

    if (hasRelevantChange) {
      this.loadUnits();
    }
  }

  onValueChange(value: number[]) {
    this.value = Array.isArray(value) ? value : [];
    this.onChange(value);
    this.onTouched();
  }

  writeValue(value: number[]): void {
    this.value = Array.isArray(value) ? value : [];
    // Reload units để đảm bảo selected units có trong danh sách
    if (this.unitCache.getAllUnits().length > 0) {
      this.loadUnits();
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

  private loadUnits() {
    const filteredUnits = this.unitCache.filterUnits({
      activeOnly: this.activeOnly,
      category: this.category,
      includeUnitIds: this.value || [],
    });

    // Đảm bảo selected units luôn có trong danh sách
    let finalUnits = filteredUnits;

    if (this.value && this.value.length > 0) {
      // Tìm các selected units không có trong filtered list
      const allUnits = this.unitCache.getAllUnits();
      const selectedUnits = allUnits.filter((unit) => this.value!.includes(unit.id!));

      // Thêm selected units vào đầu danh sách
      const selectedIds = selectedUnits.map((unit) => unit.id);
      const nonSelectedUnits = filteredUnits.filter((unit) => !selectedIds.includes(unit.id));

      finalUnits = [...selectedUnits, ...nonSelectedUnits];
    }

    this.unitOptions.set(finalUnits.map((u) => ({ id: u.id!, name: u.name! })));
  }

}
