import { ChangeDetectionStrategy, Component, computed, forwardRef, inject, Input, OnInit, signal } from '@angular/core';
import { ControlValueAccessor, FormsModule, NG_VALUE_ACCESSOR } from '@angular/forms';
import { CommonModule } from '@angular/common';
import { AutoComplete } from 'primeng/autocomplete';
import { MasterDataRegistryService } from '@/indexeddb/config/service/master-data-registry.service';
import { calculateScrollHeight } from '@/indexeddb/config/components/base/scroll-height.util';
import { TranslateService } from '@ngx-translate/core';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-base-form-control-indexed',
  standalone: true,
  imports: [CommonModule, FormsModule, AutoComplete],
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => BaseFormControlIndexed),
      multi: true,
    },
  ],
  template: `
    <p-autocomplete
      [ngModel]="displayValue()"
      (ngModelChange)="onDisplayValueChange($event)"
      [suggestions]="filteredNames()"
      (completeMethod)="filterItems($event)"
      (onSelect)="onItemSelect($event)"
      (onClear)="onClear()"
      (onBlur)="onBlur()"
      [dropdown]="true"
      [minQueryLength]="2"
      [showClear]="true"
      [appendTo]="appendTo"
      [emptyMessage]="emptyFilterMessage"
      class="w-full"
    >
    </p-autocomplete>
  `,
})
export class BaseFormControlIndexed<
  T extends {
    id?: number;
    status?: number;
    [key: string]: any;
  },
>
  implements ControlValueAccessor, OnInit
{
  @Input() tableName!: string;
  @Input() optionLabel: string = 'name';
  @Input() activeOnly: boolean = false;
  @Input() allowedStatuses: number[] = [1];
  @Input() appendTo: any = 'body';
  @Input() emptyFilterMessage: string = '';
  displayValue = signal('');
  filteredNames = signal<string[]>([]);
  value = signal<number | null>(null);
  protected registry = inject(MasterDataRegistryService);
  protected translate = inject(TranslateService);
  items = computed(() => {
    const service = this.registry.getTableService(this.tableName);
    const allItems = (service?.getAll()() || []) as T[];

    if (this.activeOnly) {
      return allItems.filter((item) => item.status === this.allowedStatuses[0]);
    }

    return allItems.filter((item) => this.allowedStatuses.includes(item.status!));
  });
  protected readonly calculateScrollHeight = calculateScrollHeight;

  async ngOnInit() {
    if (!this.emptyFilterMessage) {
      this.emptyFilterMessage = this.translate.instant('common.label.no_results');
    }
    await this.registry.loadTableOnDemand(this.tableName);
    this.updateDisplayValue();
  }

  filterItems(event: any) {
    const query = (event.query || '').toLowerCase();
    const filtered = this.items()
      .filter((item) => item[this.optionLabel] && item[this.optionLabel].toLowerCase().includes(query))
      .map((item) => item[this.optionLabel]);
    this.filteredNames.set(filtered);
  }

  onItemSelect(event: any) {
    const name = event.value || event;
    const selected = this.items().find((item) => item[this.optionLabel] === name);
    if (selected) {
      this.writeValue(selected.id!);
      this.onChange(selected.id!);
      this.onTouched();
    }
  }

  writeValue(value: number | null): void {
    this.value.set(value);
    this.updateDisplayValue();
  }

  registerOnChange(fn: (value: number | null) => void): void {
    this.onChange = fn;
  }

  registerOnTouched(fn: () => void): void {
    this.onTouched = fn;
  }

  onDisplayValueChange(value: string) {
    this.displayValue.set(value);
  }

  onClear() {
    this.writeValue(null);
    this.onChange(null);
    this.onTouched();
  }

  onBlur() {
    this.updateDisplayValue();
  }

  private onChange = (value: number | null) => {};

  private onTouched = () => {};

  private updateDisplayValue() {
    const selected = this.items().find((item) => item.id === this.value());
    this.displayValue.set(selected?.[this.optionLabel] || '');
  }
}
