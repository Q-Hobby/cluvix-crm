import { ChangeDetectionStrategy, Component, computed, EventEmitter, inject, Input, OnInit, Output } from '@angular/core';

import { FormsModule } from '@angular/forms';
import { MasterDataRegistryService } from '@/indexeddb/config/service/master-data-registry.service';
import { AutoComplete } from 'primeng/autocomplete';
import { calculateScrollHeight } from '@/indexeddb/config/components/base/scroll-height.util';
import { TranslateService } from '@ngx-translate/core';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-base-autocomplete-indexed',
  standalone: true,
  imports: [FormsModule, AutoComplete],
  template: `
    <p-autocomplete
      [(ngModel)]="displayValue"
      [suggestions]="filteredNames"
      (completeMethod)="filterItems($event)"
      (onSelect)="onNameSelect($event)"
      (onClear)="onClear()"
      (onBlur)="onBlur()"
      [dropdown]="true"
      [showClear]="true"
      [appendTo]="appendTo"
      [emptyMessage]="emptyFilterMessage"
      [minQueryLength]="2"
      class="w-full"
    >
    </p-autocomplete>
  `,
})
export class BaseAutoCompleteIndexed<
  T extends { id?: number; status?: number; [key: string]: any },
> implements OnInit {
  @Input() value: number | null = null;
  @Input() tableName!: string;
  @Input() optionLabel: string = 'name';
  @Input() activeOnly: boolean = false;
  @Input() allowedStatuses: number[] = [1];
  @Input() appendTo: any = 'body';
  @Input() emptyFilterMessage: string = '';
  @Output() valueChange = new EventEmitter<number | null>();
  filteredNames: string[] = [];
  displayValue: string = '';
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
    this.filteredNames = this.items()
      .filter((item) => item[this.optionLabel] && item[this.optionLabel].toLowerCase().includes(query))
      .map((item) => item[this.optionLabel]);
  }

  onNameSelect(event: any) {
    const name = event.value || event;
    const selected = this.items().find((item) => item[this.optionLabel] === name);
    if (selected) {
      this.value = selected.id ?? null;
      this.displayValue = selected[this.optionLabel];
      this.valueChange.emit(selected.id ?? null);
    }
  }

  onClear() {
    this.value = null;
    this.displayValue = '';
    this.valueChange.emit(null);
  }

  onBlur() {
    this.updateDisplayValue();
  }

  private updateDisplayValue() {
    const selected = this.items().find((item) => item.id === this.value);
    this.displayValue = selected?.[this.optionLabel] || '';
  }
}
