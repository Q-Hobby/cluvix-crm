import { ChangeDetectionStrategy, Component, EventEmitter, inject, Input, OnInit, Output, signal } from '@angular/core';

import { FormsModule } from '@angular/forms';
import { MultiSelect } from 'primeng/multiselect';
import { NoteTemplateCacheService } from './note-template-cache.service';
import { calculateScrollHeight } from '@/indexeddb/config/components/base/scroll-height.util';
import { NoteTemplateModel } from '@/core/models/config/marketing/note-template.model';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-note-template-multiselect',
  standalone: true,
  imports: [FormsModule, MultiSelect],
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
      [virtualScroll]="items().length > 20"
      [virtualScrollItemSize]="38"
      [scrollHeight]="calculateScrollHeight(items().length)"
      class="w-full"
    />
  `,
})
export class NoteTemplateMultiselect implements OnInit {
  @Input() selectedIds: number[] = [];
  @Output() selectedIdsChange = new EventEmitter<number[]>();
  items = signal<NoteTemplateModel[]>([]);
  protected readonly calculateScrollHeight = calculateScrollHeight;
  private cache = inject(NoteTemplateCacheService);

  async ngOnInit() {
    await this.cache.ensureNoteTemplateLoaded();
    this.loadItems();
  }

  onSelectionChange(event: any) {
    this.selectedIdsChange.emit(this.selectedIds);
  }

  private loadItems() {
    let allItems = this.cache.getAll();

    // Đảm bảo selected items luôn có trong danh sách
    let finalItems = allItems;

    if (this.selectedIds && this.selectedIds.length > 0) {
      const selectedItems = allItems.filter((item) => this.selectedIds.includes(item.id!));

      const selectedItemIds = selectedItems.map((item) => item.id);
      const nonSelectedItems = allItems.filter((item) => !selectedItemIds.includes(item.id));

      finalItems = [...selectedItems, ...nonSelectedItems];
    }

    this.items.set(finalItems);
  }
}
