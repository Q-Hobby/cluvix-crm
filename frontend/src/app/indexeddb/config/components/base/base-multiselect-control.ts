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
} from '@angular/core';
import { ControlValueAccessor, FormsModule, NG_VALUE_ACCESSOR } from '@angular/forms';

import { MultiSelect } from 'primeng/multiselect';
import { calculateScrollHeight } from '@/indexeddb/config/components/base/scroll-height.util';
import { TranslateService } from '@ngx-translate/core';

export interface SelectOption {
  id: number;
  name: string;
}

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-base-multiselect-control',
  standalone: true,
  imports: [FormsModule, MultiSelect],
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => BaseMultiSelectControlComponent),
      multi: true,
    },
  ],
  template: `
    <p-multiselect
      [options]="displayedOptions"
      [(ngModel)]="value"
      (ngModelChange)="onSelectionChange($event)"
      (onFilter)="onFilter($event)"
      optionLabel="name"
      optionValue="id"
      [showClear]="true"
      [style]="{ minWidth: '100%' }"
      [appendTo]="appendTo"
      [emptyFilterMessage]="emptyFilterMessage"
      [resetFilterOnHide]="resetFilterOnHide"
      [filter]="true"
      [scrollHeight]="calculateScrollHeight(displayedOptions.length)"
      [virtualScroll]="displayedOptions.length > 20"
      [virtualScrollItemSize]="38"
      [maxSelectedLabels]="2"
      class="w-full"
    >
    </p-multiselect>
  `,
})
export class BaseMultiSelectControlComponent implements ControlValueAccessor, OnInit, OnChanges {
  @Input() appendTo: any = 'body';
  @Input() emptyFilterMessage = '';
  @Input() resetFilterOnHide = true;
  @Input() options: SelectOption[] = [];
  @Output() valueChange = new EventEmitter<number[] | null>();

  displayedOptions: SelectOption[] = [];
  value: number[] | null = [];
  protected readonly calculateScrollHeight = calculateScrollHeight;
  protected translate = inject(TranslateService);
  private allOptions: SelectOption[] = [];

  ngOnInit() {
    if (!this.emptyFilterMessage) {
      this.emptyFilterMessage = this.translate.instant('common.label.no_results');
    }
    this.updateOptions();
  }

  ngOnChanges() {
    this.updateOptions();
  }

  onFilter(event: any) {
    const query = (event.filter || '').toLowerCase() || '';
    if (query) {
      // Filter from all options
      this.displayedOptions = this.allOptions.filter((option) =>
        option.name && option.name.toLowerCase().includes(query),
      );
    } else {
      // Reset to limited display
      this.updateDisplayedOptions();
    }
  }

  onSelectionChange(value: number[]) {
    // KHÔNG gọi writeValue() ở đây — writeValue chỉ dành cho lúc parent form đẩy giá trị
    // xuống (CVA). Gọi lại nó ở mỗi lần user chọn/bỏ chọn sẽ dựng lại mảng displayedOptions
    // (reorder "selected trước") mới mỗi click, khiến [options] đổi reference liên tục và
    // làm header "chọn tất cả" của p-multiselect (kèm virtualScroll) mất đồng bộ với item bên dưới.
    this.value = Array.isArray(value) ? value : [];
    this.onChange(value);
    this.onTouched();
  }

  writeValue(value: number[]): void {
    this.value = Array.isArray(value) ? value : [];
    // Cập nhật displayed options để đảm bảo selected items có trong danh sách
    if (this.allOptions.length > 0) {
      this.updateDisplayedOptions();
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

  private updateOptions() {
    this.allOptions = [...this.options];
    this.updateDisplayedOptions();
  }

  private updateDisplayedOptions() {
    // Đảm bảo selected items luôn có trong danh sách
    let finalOptions = this.allOptions;

    if (this.value && this.value.length > 0) {
      // Tìm các selected items không có trong allOptions
      const selectedOptions = this.allOptions.filter((option) => this.value!.includes(option.id));

      // Thêm selected items vào đầu danh sách
      const selectedIds = selectedOptions.map((option) => option.id);
      const nonSelectedOptions = this.allOptions.filter(
        (option) => !selectedIds.includes(option.id),
      );

      finalOptions = [...selectedOptions, ...nonSelectedOptions];
    }

    // Load tất cả items để filter được đúng
    // Virtual scroll sẽ xử lý performance
    this.displayedOptions = finalOptions;
  }
}
