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
import { TranslatePipe, TranslateService } from '@ngx-translate/core';
import { UserCacheService } from './user-cache.service';
import { calculateScrollHeight } from '@/indexeddb/config/components/base/scroll-height.util';
import { UserProfileModel } from '@/core/models/user-profile.model';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-user-multiselect-form-control',
  standalone: true,
  imports: [FormsModule, MultiSelect, TranslatePipe],
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => UserMultiSelectFormControl),
      multi: true,
    },
  ],
  template: `
    <p-multiselect
      [(ngModel)]="selectedValues"
      [options]="items()"
      optionLabel="fullname"
      optionValue="id"
      [showClear]="true"
      [filter]="true"
      filterBy="fullname"
      [emptyMessage]="'common.label.no_results' | translate"
      appendTo="body"
      [maxSelectedLabels]="2"
      [virtualScroll]="items().length > 20"
      [virtualScrollItemSize]="38"
      [scrollHeight]="calculateScrollHeight(items().length)"
      class="w-full"
      (onChange)="onSelectionChange($event)"
      (onClear)="onClear()"
    />
  `,
})
/**
 * UserMultiSelectFormControl khác với UserMultiSelect ở chỗ:
 * ControlValueAccessor : UserMultiSelectFormControl implement ControlValueAccessor để hoạt động với Angular forms, UserMultiSelect chỉ là component thường
 * Filtering options : UserMultiSelectFormControl có thêm các filter như department, department, branchId
 * Invalid state : UserMultiSelectFormControl có @Input invalid để hiển thị validation state
 */
export class UserMultiSelectFormControl implements ControlValueAccessor, OnInit, OnChanges {
  @Input() activeOnly: boolean = false;
  @Input() allowedStatuses: number[] = [9, 10];
  @Input() department?: number;
  @Input() departments?: number[];
  @Input() branchId?: number;
  @Input() includeNull: boolean = false;
  @Output() valueChange = new EventEmitter<number[] | null>();
  selectedValues: number[] | null = null;
  items = signal<UserProfileModel[]>([]);
  protected readonly calculateScrollHeight = calculateScrollHeight;
  private cache = inject(UserCacheService);
  private translate = inject(TranslateService);
  private cachedFilteredItems: UserProfileModel[] = [];
  private lastFilterKey = '';

  ngOnChanges(changes: SimpleChanges) {
    if (
      (changes['activeOnly'] && !changes['activeOnly'].firstChange) ||
      (changes['department'] && !changes['department'].firstChange) ||
      (changes['branchId'] && !changes['branchId'].firstChange) ||
      (changes['includeNull'] && !changes['includeNull'].firstChange)
    ) {
      // Clear cache khi filter parameters thay đổi
      this.lastFilterKey = '';
      this.cachedFilteredItems = [];
      this.loadItems();
    }
  }

  async ngOnInit() {
    await this.cache.loadUsers();
    this.loadItems();
  }

  onSelectionChange(event: any) {
    const value = event.value || null;
    this.onChange(value);
    this.onTouched();
    this.valueChange.emit(value);
  }

  writeValue(value: number[] | null): void {
    this.selectedValues = Array.isArray(value) ? value : [];
    // Reload items để đảm bảo selected items có trong danh sách.
    // Clear lastFilterKey để buộc filter lại — tránh trường hợp ngOnInit đã chạy
    // loadItems() trước writeValue (cache hit) và cachedFilteredItems không chứa
    // các selected users (đã chuyển chi nhánh). Nếu không clear, lần gọi này
    // dùng cache cũ → selected users không được thêm vào finalItems.
    if (this.cache.getAll().length > 0) {
      this.lastFilterKey = '';
      this.cachedFilteredItems = [];
      this.loadItems();
    }
  }

  onClear() {
    this.selectedValues = null;
    this.onChange(null);
    this.onTouched();
    this.valueChange.emit(null);
  }

  registerOnChange(fn: (value: number[] | null) => void): void {
    this.onChange = fn;
  }

  registerOnTouched(fn: () => void): void {
    this.onTouched = fn;
  }

  private onChange = (value: number[] | null) => {};

  private onTouched = () => {};

  private loadItems(): void {
    // Tạo cache key dựa trên các filter parameters
    const filterKey = `${this.department || ''}_${(this.departments || []).join(',')}_${this.branchId || ''}_${this.activeOnly}_${this.allowedStatuses.join(',')}_v${this.cache.dataVersion}`;

    // Chỉ filter lại khi các parameters thay đổi
    let filteredItems: UserProfileModel[];
    if (this.lastFilterKey !== filterKey) {
      filteredItems = this.cache.filterUsers({
        department: this.department,
        departments: this.departments,
        branchId: this.branchId,
        activeOnly: this.activeOnly,
        allowedStatuses: this.allowedStatuses,
        autoFallback: true,
      });
      this.cachedFilteredItems = filteredItems;
      this.lastFilterKey = filterKey;
    } else {
      filteredItems = this.cachedFilteredItems;
    }

    // Đảm bảo selected items luôn có trong danh sách
    let finalItems = filteredItems;

    if (this.selectedValues && this.selectedValues.length > 0) {
      const selectedItems = this.cache
        .getAllUsersUnfiltered()
        .filter((item) => this.selectedValues!.includes(item.id!));

      const selectedIds = selectedItems.map((item) => item.id);
      const nonSelectedItems = filteredItems.filter((item) => !selectedIds.includes(item.id));

      finalItems = [...selectedItems, ...nonSelectedItems];
    }

    // Prepend sentinel option khi includeNull = true
    if (this.includeNull) {
      const nullOption = { id: 0, fullname: this.translate.instant('common.label.unassigned') } as UserProfileModel;
      finalItems = [nullOption, ...finalItems.filter((i) => i.id !== 0)];
    }

    // User thiếu `fullname` → PrimeNG in chữ "null" ở chip/option; lùi về nickname/code/email/#id.
    finalItems = finalItems.map((u) =>
      u.fullname ? u : { ...u, fullname: u.nickname || u.code || u.email || `#${u.id}` },
    );

    // Load tất cả items để filter được đúng trong PrimeNG MultiSelect
    // PrimeNG sẽ tự động virtual scroll nên không ảnh hưởng performance
    this.items.set(finalItems);
  }
}
