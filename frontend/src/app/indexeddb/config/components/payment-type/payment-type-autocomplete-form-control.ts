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
import { PaymentTypeCacheService } from './payment-type-cache.service';
import { CardTypeModel } from '@/core/models/config/accountant/card-type.model';
import { TranslateService } from '@ngx-translate/core';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-payment-type-autocomplete-form-control',
  standalone: true,
  imports: [FormsModule, AutoComplete],
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => PaymentTypeAutocompleteFormControl),
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
      optionLabel="name"
      [showClear]="true"
      appendTo="body"
      [emptyMessage]="emptyMessage"
      class="w-full"
    />
  `,
})
export class PaymentTypeAutocompleteFormControl implements ControlValueAccessor, OnInit, OnChanges {
  protected emptyMessage = '';
  private translate = inject(TranslateService);
  @Input() paymentMethodId: number | null = null;
  @Output() valueChange = new EventEmitter<number | null>();
  selectedItem: CardTypeModel | null = null;
  filteredItems = signal<CardTypeModel[]>([]);
  private cache = inject(PaymentTypeCacheService);
  private previousPaymentMethodId: number | null = null;

  async ngOnInit() {
    this.emptyMessage = this.translate.instant('common.label.no_results');
    await this.cache.ensurePaymentTypesLoaded();
    this.previousPaymentMethodId = this.paymentMethodId;
  }

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['paymentMethodId'] && !changes['paymentMethodId'].firstChange) {
      if (this.paymentMethodId !== this.previousPaymentMethodId) {
        this.selectedItem = null;
        this.onChange(null);
        this.valueChange.emit(null);
        this.previousPaymentMethodId = this.paymentMethodId;
      }
    }
  }

  filterItems(event: any) {
    const query = (event.query || '').toLowerCase();
    let items = this.paymentMethodId ? this.cache.getByPaymentMethodId(this.paymentMethodId) : [];

    const filtered = items.filter((item) => item.name && item.name.toLowerCase().includes(query));
    this.filteredItems.set(filtered);
  }

  onSelect(event: any) {
    const value = event.value?.id || null;
    this.onChange(value);
    this.onTouched();
    this.valueChange.emit(value);
  }

  onClear() {
    this.selectedItem = null;
    this.onChange(null);
    this.onTouched();
    this.valueChange.emit(null);
  }

  onBlur() {
    if (typeof this.selectedItem === 'string' || (this.selectedItem && !this.selectedItem.id)) {
      this.selectedItem = null;
    }
  }

  writeValue(id: number | null): void {
    this.selectedItem = id ? this.cache.getById(id) || null : null;
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
