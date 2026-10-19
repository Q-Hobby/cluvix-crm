import {
  ChangeDetectionStrategy,
  booleanAttribute,
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

import { AutoComplete } from 'primeng/autocomplete';
import { TranslatePipe } from '@ngx-translate/core';
import { UserCacheService } from './user-cache.service';
import { UserProfileModel } from '@/core/models/user-profile.model';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-user-form-control',
  standalone: true,
  imports: [FormsModule, AutoComplete, TranslatePipe],
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => UserFormControl),
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
      optionLabel="fullname"
      [showClear]="true"
      appendTo="body"
      [emptyMessage]="'common.label.no_results' | translate"
      [ariaLabel]="ariaLabel"
      class="w-full"
    />
  `,
})
/**
 * UserFormControl khác với UserAutoComplete ở chỗ:
 * ControlValueAccessor : UserFormControl implement ControlValueAccessor để hoạt động với Angular forms, UserAutoComplete chỉ là component thường
 * Filtering options : UserFormControl có thêm các filter như department, department, branchId
 * Invalid state : UserFormControl có @Input invalid để hiển thị validation state
 */
export class UserFormControl implements ControlValueAccessor, OnInit, OnChanges {
  @Input() activeOnly: boolean = false;
  @Input() allowedStatuses: number[] = [9, 10];
  @Input({ transform: booleanAttribute }) invalid!: boolean;
  @Input() department?: number;
  @Input() departments?: number[];
  @Input() branchId?: number;
  /**
   * Loại hẳn các nhân viên này khỏi danh sách chọn — dùng khi bảng đích có UNIQUE trên `user_id`
   * (vd credential liên thông DQG: mỗi bác sĩ đúng 1 dòng), chọn trùng sẽ bị BE trả 409.
   * Mặc định rỗng ⇒ mọi màn đang dùng component này không đổi hành vi.
   */
  @Input() excludeIds: number[] = [];
  /**
   * Tên đọc được cho ô chọn nhân sự (B.19b): `p-autocomplete` không có nhãn liên kết nên screen
   * reader chỉ đọc "combobox, blank". Chỗ gọi truyền chuỗi ĐÃ dịch (i18n) xuống.
   * Không truyền ⇒ `undefined` ⇒ không phát attribute ⇒ hành vi y như trước.
   */
  @Input() ariaLabel?: string;
  @Output() valueChange = new EventEmitter<number | null>();
  selectedItem: UserProfileModel | null = null;
  filteredItems = signal<UserProfileModel[]>([]);
  private cache = inject(UserCacheService);
  private cdr = inject(ChangeDetectorRef);
  private cachedFilteredItems: UserProfileModel[] = [];
  private lastFilterKey = '';

  ngOnChanges(changes: SimpleChanges) {
    if (
      (changes['activeOnly'] && !changes['activeOnly'].firstChange) ||
      (changes['department'] && !changes['department'].firstChange) ||
      (changes['branchId'] && !changes['branchId'].firstChange) ||
      (changes['excludeIds'] && !changes['excludeIds'].firstChange)
    ) {
      // Invalidate cache so getFilteredItems() picks up new params
      this.lastFilterKey = '';
      this.cachedFilteredItems = [];

      // Only clear selection if the current user is no longer in the new filtered list
      if (this.selectedItem) {
        const stillValid = this.getFilteredItems().some((u) => u.id === this.selectedItem!.id);
        if (!stillValid) {
          this.selectedItem = null;
          this.onChange(null);
        }
      }
    }
  }

  async ngOnInit() {
    await this.cache.loadUsers();
  }

  filterItems(event: any) {
    const query = (event?.query || '').toLowerCase();
    let items = this.getFilteredItems();

    if (!query) {
      // Không có query - hiển thị giới hạn để tránh lag
      items = items.filter((item) => item?.fullname);
      if (this.selectedItem?.fullname && !items.find((item) => item.id === this.selectedItem!.id)) {
        items = [this.selectedItem, ...items];
      }
      this.filteredItems.set(items.slice(0, 100)); // Giới hạn 100 items khi dropdown
      return;
    }

    // Có query - filter trong toàn bộ danh sách
    const filtered = items.filter((item) => {
      if (!item?.fullname) return false;
      const fullname = item.fullname.toLowerCase();
      const nickname = (item?.nickname || '').toLowerCase();
      return fullname.includes(query) || nickname.includes(query);
    });

    if (this.selectedItem?.fullname) {
      const selectedFullname = this.selectedItem.fullname.toLowerCase();
      const selectedNickname = (this.selectedItem?.nickname || '').toLowerCase();

      if (
        (selectedFullname.includes(query) || selectedNickname.includes(query)) &&
        !filtered.find((item) => item.id === this.selectedItem!.id)
      ) {
        filtered.unshift(this.selectedItem);
      }
    }

    this.filteredItems.set(filtered.slice(0, 200)); // Giới hạn 200 kết quả search
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
    // Use setTimeout to avoid ExpressionChangedAfterItHasBeenCheckedError
    // when value is set during initialization
    setTimeout(() => {
      this.selectedItem = id
        ? this.cache.getAllUsersUnfiltered().find((u) => u.id === id) || null
        : null;
      this.cdr.markForCheck();
    });
  }

  registerOnChange(fn: (value: number | null) => void): void {
    this.onChange = fn;
  }

  registerOnTouched(fn: () => void): void {
    this.onTouched = fn;
  }

  private onChange = (value: number | null) => {};

  private onTouched = () => {};

  private getFilteredItems(): UserProfileModel[] {
    // Tạo cache key dựa trên các filter parameters
    const filterKey = `${this.department || ''}_${(this.departments || []).join(',')}_${this.branchId || ''}_${this.activeOnly}_${(this.allowedStatuses || []).join(',')}_x${(this.excludeIds || []).join(',')}_v${this.cache.dataVersion}`;

    // Chỉ filter lại khi các parameters thay đổi
    if (this.lastFilterKey !== filterKey) {
      this.cachedFilteredItems = this.cache.filterUsers({
        department: this.department,
        departments: this.departments,
        branchId: this.branchId,
        activeOnly: this.activeOnly,
        allowedStatuses: this.allowedStatuses || [],
        autoFallback: true,
      });
      // Loại trừ nằm SAU filterUsers và nằm TRONG khối cache: `excludeIds` đã có mặt trong
      // `filterKey` nên đổi danh sách loại trừ sẽ tính lại, không trả về bản cache cũ.
      const ex = this.excludeIds || [];
      if (ex.length) {
        this.cachedFilteredItems = this.cachedFilteredItems.filter((u) => !ex.includes(u.id as number));
      }
      this.lastFilterKey = filterKey;
    }

    return this.cachedFilteredItems;
  }
}
