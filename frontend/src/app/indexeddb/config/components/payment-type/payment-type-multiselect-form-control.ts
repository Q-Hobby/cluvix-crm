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
import { PaymentTypeCacheService } from './payment-type-cache.service';
import { CardTypeModel } from '@/core/models/config/accountant/card-type.model';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-payment-type-multiselect-form-control',
  standalone: true,
  imports: [FormsModule, MultiSelect],
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => PaymentTypeMultiselectFormControl),
      multi: true,
    },
  ],
  template: `
    <p-multiselect
      [(ngModel)]="selectedIds"
      [options]="items()"
      (onChange)="onSelectionChange($event)"
      [filter]="true"
      [showClear]="true"
      optionLabel="name"
      optionValue="id"
      appendTo="body"
      [maxSelectedLabels]="2"
      class="w-full"
      (onClear)="onClear()"
    />
  `,
})
export class PaymentTypeMultiselectFormControl implements ControlValueAccessor, OnInit, OnChanges {
  @Input() paymentMethodId: number | null = null;
  @Output() valueChange = new EventEmitter<number[] | null>();
  selectedIds: number[] | null = [];
  items = signal<CardTypeModel[]>([]);
  private cache = inject(PaymentTypeCacheService);

  async ngOnInit() {
    await this.cache.ensurePaymentTypesLoaded();
    this.loadItems();
  }

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['paymentMethodId']) {
      this.loadItems();
    }
  }

  onSelectionChange(event: any) {
    this.onChange(this.selectedIds ?? []);
    this.onTouched();
  }

  writeValue(value: number[] | null): void {
    this.selectedIds = Array.isArray(value) ? value : [];
  }

  onClear() {
    this.selectedIds = null;
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

  private loadItems() {
    const items = this.paymentMethodId
      ? this.cache.getByPaymentMethodId(this.paymentMethodId)
      : this.cache.getAll();
    this.items.set(items);
  }
}
