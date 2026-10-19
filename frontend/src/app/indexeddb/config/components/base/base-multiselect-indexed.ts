import { ChangeDetectionStrategy, Component, computed, EventEmitter, inject, Input, OnInit, Output } from '@angular/core';

import { FormsModule } from '@angular/forms';
import { MasterDataRegistryService } from '@/indexeddb/config/service/master-data-registry.service';
import { MultiSelect } from 'primeng/multiselect';
import { TranslateService } from '@ngx-translate/core';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-base-multiselect-indexed',
  standalone: true,
  imports: [FormsModule, MultiSelect],
  template: `
    <p-multiselect
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
      [filterBy]="optionLabel"
      class="w-full"
    >
    </p-multiselect>
  `,
})
export class BaseMultiSelectIndexed<T extends { id?: number; status?: number }> implements OnInit {
  @Input() value: number[] = [];
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
  @Output() valueChange = new EventEmitter<number[]>();

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

  onSelectionChange(value: number[]) {
    this.value = Array.isArray(value) ? value : [];
    this.valueChange.emit(value);
  }
}
