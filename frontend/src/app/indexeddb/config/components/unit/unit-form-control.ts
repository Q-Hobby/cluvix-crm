import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
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
import { UnitCacheService } from './unit-cache.service';
import { TranslateService } from '@ngx-translate/core';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-unit-form-control',
  standalone: true,
  imports: [FormsModule, AutoComplete],
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => UnitFormControl),
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
      class="w-full"
    />
  `,
})
export class UnitFormControl implements ControlValueAccessor, OnInit, OnChanges {
  @Input() activeOnly: boolean = false;
  @Input() category?: number;
  @Input() appendTo: any = 'body';
  @Input() emptyFilterMessage: string = '';

  value: number | null = null;
  filteredNames: string[] = [];
  displayValue: string = '';
  units = signal<any[]>([]);
  private cdr = inject(ChangeDetectorRef);
  private translate = inject(TranslateService);

  constructor(private unitCache: UnitCacheService) {}

  async ngOnInit() {
    if (!this.emptyFilterMessage) {
      this.emptyFilterMessage = this.translate.instant('common.label.no_results');
    }
    await this.unitCache.loadUnits();
    this.loadUnits();
    this.updateDisplayValue();
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

  filterItems(event: any) {
    const query = (event.query || '').toLowerCase();
    this.filteredNames = this.units()
      .filter((unit) => unit.name && unit.name.toLowerCase().includes(query))
      .map((unit) => unit.name);
  }

  onNameSelect(event: any) {
    const name = event.value || event;
    const selected = this.units().find((unit) => unit.name === name);
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
    // Reload units để đảm bảo selected unit có trong danh sách
    if (this.unitCache.getAllUnits().length > 0) {
      this.loadUnits();
    }
    this.updateDisplayValue();
    // OnPush component không tự re-render khi parent form patchValue → ép markForCheck
    this.cdr.markForCheck();
  }

  registerOnChange(fn: (value: number | null) => void): void {
    this.onChange = fn;
  }

  registerOnTouched(fn: () => void): void {
    this.onTouched = fn;
  }

  private onChange = (value: number | null) => {};

  private onTouched = () => {};

  private loadUnits() {
    const filteredUnits = this.unitCache.filterUnits({
      activeOnly: this.activeOnly,
      category: this.category,
      includeUnitId: this.value || undefined,
    });

    // Đảm bảo selected unit luôn có trong danh sách
    let finalUnits = filteredUnits;

    if (this.value) {
      const allUnits = this.unitCache.getAllUnits();
      const selectedUnit = allUnits.find((unit) => unit.id === this.value);

      if (selectedUnit) {
        // Thêm selected unit vào đầu danh sách
        const nonSelectedUnits = filteredUnits.filter((unit) => unit.id !== this.value);
        finalUnits = [selectedUnit, ...nonSelectedUnits];
      }
    }

    this.units.set(finalUnits);
  }

  private updateDisplayValue() {
    const selected = this.units().find((unit) => unit.id === this.value);
    this.displayValue = selected?.name || '';
  }
}
