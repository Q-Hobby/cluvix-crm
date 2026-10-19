import { ChangeDetectionStrategy, Component, forwardRef, Input, OnInit } from '@angular/core';
import { ControlValueAccessor, NG_VALUE_ACCESSOR } from '@angular/forms';
import { BaseAutoCompleteIndexed } from '@/indexeddb/config/components/base/base-autocomplete-indexed';
import { UnitCacheService } from './unit-cache.service';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-unit-select',
  standalone: true,
  imports: [BaseAutoCompleteIndexed],
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => UnitSelect),
      multi: true,
    },
  ],
  template: `
    <app-base-autocomplete-indexed
      [tableName]="'units'"
      [optionLabel]="'name'"
      [activeOnly]="activeOnly"
      [value]="value"
      (valueChange)="onValueChange($event)"
    />
  `,
})
export class UnitSelect implements ControlValueAccessor, OnInit {
  @Input() activeOnly: boolean = false;

  value: number | null = null;

  constructor(private unitCache: UnitCacheService) {}

  async ngOnInit() {
    await this.unitCache.loadUnits();
  }

  onValueChange(value: number | null) {
    this.value = value;
    this.onChange(value);
    this.onTouched();
  }

  writeValue(value: number | null): void {
    this.value = value;
  }

  registerOnChange(fn: (value: number | null) => void): void {
    this.onChange = fn;
  }

  registerOnTouched(fn: () => void): void {
    this.onTouched = fn;
  }

  private onChange = (value: number | null) => {};

  private onTouched = () => {};
}
