import { ChangeDetectionStrategy, Component, EventEmitter, inject, Input, OnInit, Output, signal } from '@angular/core';

import { FormsModule } from '@angular/forms';
import { MultiSelect } from 'primeng/multiselect';
import { AppointmentIsAcceptedCacheService } from './appointment-is-accepted-cache.service';
import { calculateScrollHeight } from '@/indexeddb/config/components/base/scroll-height.util';
import { AppointmentIsAcceptedModel } from '@/core/models/config/appointment/appointment-is-accepted.model';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-appointment-is-accepted-multiselect',
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
export class AppointmentIsAcceptedMultiselect implements OnInit {
  @Input() selectedIds: number[] = [];
  @Output() selectedIdsChange = new EventEmitter<number[]>();
  items = signal<AppointmentIsAcceptedModel[]>([]);
  protected readonly calculateScrollHeight = calculateScrollHeight;
  private cache = inject(AppointmentIsAcceptedCacheService);

  async ngOnInit() {
    await this.cache.ensureAppointmentIsAcceptedsLoaded();
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
      const selectedItemsData = allItems.filter((item) => this.selectedIds.includes(item.id!));

      const selectedItemIds = selectedItemsData.map((item) => item.id);
      const nonSelectedItems = allItems.filter((item) => !selectedItemIds.includes(item.id));

      finalItems = [...selectedItemsData, ...nonSelectedItems];
    }

    this.items.set(finalItems);
  }
}
