import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
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

import { CascadeSelect } from 'primeng/cascadeselect';
import { AppointmentNotBookedReasonsCacheService } from './appointment-not-booked-reasons-cache.service';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-appointment-not-booked-reasons-cascade-form-control',
  standalone: true,
  imports: [FormsModule, CascadeSelect],
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => AppointmentNotBookedReasonsCascadeFormControl),
      multi: true,
    },
  ],
  template: `
    <p-cascadeselect
      [(ngModel)]="selectedValue"
      [options]="options()"
      optionLabel="name"
      optionValue="id"
      optionGroupLabel="name"
      [optionGroupChildren]="['children']"
      [showClear]="true"
      [style]="style"
      [size]="size"
      [autofocus]="false"
      appendTo="body"
      class="w-full"
      (onChange)="onSelectionChange($event)"
      (onClear)="onClear()"
    />
  `,
})
export class AppointmentNotBookedReasonsCascadeFormControl
  implements ControlValueAccessor, OnInit, OnChanges
{
  @Input() style: any = {};
  @Input() size: 'small' | 'large' | undefined;
  @Input() value: number | null = null;
  @Output() valueChange = new EventEmitter<number | null>();
  selectedValue: number | null = null;
  options = signal<any[]>([]);
  private cache = inject(AppointmentNotBookedReasonsCacheService);
  private cdr = inject(ChangeDetectorRef);
  private isInitialized = false;

  async ngOnInit() {
    await this.cache.ensureReasonsLoaded();
    this.loadOptions();

    // Sử dụng requestAnimationFrame thay vì setTimeout
    requestAnimationFrame(() => {
      this.selectedValue = this.value;
      this.isInitialized = true;
      this.cdr.markForCheck();
    });
  }

  ngOnChanges(changes: SimpleChanges) {
    if (changes['value'] && this.options().length > 0) {
      this.selectedValue = this.value;
      this.cdr.markForCheck();
    }
  }

  onSelectionChange(event: any) {
    const value = event.value || null;
    this.onChange(value);
    this.onTouched();

    // Chỉ emit khi user thực sự thay đổi, không phải khi init
    if (this.isInitialized) {
      this.valueChange.emit(value);
    }
  }

  onClear() {
    this.selectedValue = null;
    this.onChange(null);
    this.onTouched();

    // Chỉ emit khi user thực sự clear
    if (this.isInitialized) {
      this.valueChange.emit(null);
    }
  }

  writeValue(value: number | null): void {
    this.selectedValue = value;
  }

  registerOnChange(fn: (value: number | null) => void): void {
    this.onChange = fn;
  }

  registerOnTouched(fn: () => void): void {
    this.onTouched = fn;
  }

  private onChange = (value: number | null) => {};

  private onTouched = () => {};

  private loadOptions(): void {
    const cascadeOptions = this.cache.getCascadeOptions();
    this.options.set(cascadeOptions);
  }
}
