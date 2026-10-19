import { ChangeDetectionStrategy, Component, computed, EventEmitter, inject, Input, OnInit, Output } from '@angular/core';

import { FormsModule } from '@angular/forms';
import { MasterDataRegistryService } from '@/indexeddb/config/service/master-data-registry.service';
import { Select } from 'primeng/select';
import { TranslateService } from '@ngx-translate/core';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-base-select-indexed',
  standalone: true,
  imports: [FormsModule, Select],
  template: `
    <p-select
      [options]="items()"
      [(ngModel)]="value"
      (ngModelChange)="onSelectionChange($event)"
      [optionLabel]="optionLabel"
      [optionValue]="optionValue"
      [showClear]="true"
      [appendTo]="appendTo"
      [filter]="filter || items().length >= 5"
      [resetFilterOnHide]="resetFilterOnHide"
      [virtualScroll]="virtualScroll"
      [virtualScrollItemSize]="virtualScrollItemSize"
      [emptyFilterMessage]="emptyFilterMessage"
      [placeholder]="placeholder"
      [filterBy]="optionLabel"
      class="w-full"
    >
    </p-select>
  `,
})
export class BaseSelectIndexed<T extends { id?: number; status?: number }> implements OnInit {
  @Input() value: number | null = null;
  @Input() tableName!: string;
  @Input() optionLabel: string = 'name';
  @Input() optionValue: string = 'id';
  @Input() activeOnly: boolean = false;
  @Input() allowedStatuses: number[] = [1];
  @Input() appendTo: any = 'body';
  @Input() filter: boolean = false;
  @Input() resetFilterOnHide: boolean = true;
  @Input() virtualScroll: boolean = false;
  @Input() virtualScrollItemSize: number = 40;
  @Input() emptyFilterMessage: string = '';
  /** Nhãn hiện khi CHƯA chọn gì. Mặc định `''` = giữ nguyên hành vi cũ (không nhãn) cho mọi chỗ đang
   *  dùng — thêm vào cho story-25 (vá QA F5): dropdown lọc người gửi phải có nhãn giống dropdown kênh. */
  @Input() placeholder: string = '';
  @Output() valueChange = new EventEmitter<number | null>();

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

  async ngOnInit() {
    if (!this.emptyFilterMessage) {
      this.emptyFilterMessage = this.translate.instant('common.label.no_results');
    }
    await this.registry.loadTableOnDemand(this.tableName);
  }

  onSelectionChange(value: number | null) {
    this.value = value;
    this.valueChange.emit(value);
  }
}
