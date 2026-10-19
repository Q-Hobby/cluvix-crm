import { ChangeDetectionStrategy, Component, EventEmitter, inject, Input, OnInit, Output, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { AutoComplete } from 'primeng/autocomplete';
import { BranchCacheService } from './branch-cache.service';
import { BranchModel } from '@/core/models/config/system/branch.model';
import { TranslateService } from '@ngx-translate/core';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-branch-autocomplete',
  standalone: true,
  imports: [CommonModule, FormsModule, AutoComplete],
  template: `
    <p-autocomplete
      [(ngModel)]="selectedItem"
      [suggestions]="filteredItems()"
      (completeMethod)="filterItems($event)"
      (onSelect)="onSelect($event)"
      (onClear)="onClear()"
      (onBlur)="onBlur()"
      [dropdown]="dropdown"
      optionLabel="name"
      [showClear]="showClear"
      [appendTo]="appendTo"
      [emptyMessage]="emptyFilterMessage"
      [minQueryLength]="0"
      class="w-full"
    >
    </p-autocomplete>
  `,
})
export class BranchAutoComplete implements OnInit {
  @Input() dropdown = true;
  @Input() showClear = true;
  @Input() appendTo: any = 'body';
  @Input() emptyFilterMessage = '';
  @Input() activeOnly: boolean = false;
  @Input() selectedValue: number | null = null;
  @Output() selectionChange = new EventEmitter<number | null>();
  selectedItem: BranchModel | null = null;
  filteredItems = signal<BranchModel[]>([]);
  private branchCache = inject(BranchCacheService);
  private translate = inject(TranslateService);

  async ngOnInit() {
    if (!this.emptyFilterMessage) {
      this.emptyFilterMessage = this.translate.instant('common.label.no_results');
    }
    await this.branchCache.loadBranches();
    this.selectedItem = this.selectedValue
      ? this.branchCache.getById(this.selectedValue) || null
      : null;
  }

  filterItems(event: any) {
    const query = (event.query || '').toLowerCase();
    let items = this.getFilteredItems();

    if (query) {
      items = items.filter((item) => item.name && item.name.toLowerCase().includes(query));
    }

    if (this.selectedItem && !items.some((item) => item.id === this.selectedItem!.id)) {
      if (!query || this.selectedItem.name && this.selectedItem.name.toLowerCase().includes(query)) {
        items = [this.selectedItem, ...items];
      }
    }

    this.filteredItems.set(items.length > 100 ? items.slice(0, 100) : items);
  }

  onSelect(event: any) {
    const value = event.value?.id || null;
    this.selectedValue = value;
    this.selectionChange.emit(value);
  }

  onClear() {
    this.selectedItem = null;
    this.selectedValue = null;
    this.selectionChange.emit(null);
  }

  onBlur() {
    if (typeof this.selectedItem === 'string' || (this.selectedItem && !this.selectedItem.id)) {
      this.selectedItem = this.selectedValue
        ? this.branchCache.getById(this.selectedValue) || null
        : null;
    }
  }

  private getFilteredItems(): BranchModel[] {
    let items = this.branchCache.filterBranchesByUser({
      activeOnly: this.activeOnly,
    });

    return items;
  }
}
