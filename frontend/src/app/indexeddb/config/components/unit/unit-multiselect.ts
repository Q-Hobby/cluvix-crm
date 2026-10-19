import { ChangeDetectionStrategy, Component, forwardRef, Input, OnInit } from '@angular/core';
import { ControlValueAccessor, NG_VALUE_ACCESSOR } from '@angular/forms';
import { BaseMultiSelectIndexed } from '@/indexeddb/config/components/base/base-multiselect-indexed';
import { UnitCacheService } from './unit-cache.service';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-unit-multiselect',
  standalone: true,
  imports: [BaseMultiSelectIndexed],
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => UnitMultiSelect),
      multi: true,
    },
  ],
  template: `
    <app-base-multiselect-indexed
      [tableName]="'units'"
      [optionLabel]="'name'"
      [activeOnly]="activeOnly"
      [value]="value"
      (valueChange)="onValueChange($event)"
    />
  `,
})
export class UnitMultiSelect implements ControlValueAccessor, OnInit {
  @Input() activeOnly: boolean = false;

  value: number[] = [];

  constructor(private unitCache: UnitCacheService) {}

  async ngOnInit() {
    await this.unitCache.loadUnits();
  }

  onValueChange(value: number[]) {
    this.value = Array.isArray(value) ? value : [];
    this.onChange(value);
    this.onTouched();
  }

  writeValue(value: number[]): void {
    this.value = Array.isArray(value) ? value : [];
  }

  registerOnChange(fn: (value: number[]) => void): void {
    this.onChange = fn;
  }

  registerOnTouched(fn: () => void): void {
    this.onTouched = fn;
  }

  private onChange = (value: number[]) => {};

  private onTouched = () => {};
}
