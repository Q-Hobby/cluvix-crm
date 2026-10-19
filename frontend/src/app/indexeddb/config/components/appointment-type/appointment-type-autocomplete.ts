import { ChangeDetectionStrategy, Component, EventEmitter, inject, Input, OnInit, Output, signal } from '@angular/core';

import { FormsModule } from '@angular/forms';
import { AutoComplete } from 'primeng/autocomplete';
import { AppointmentTypeCacheService } from './appointment-type-cache.service';
import { AppointmentTypeModel } from '@/core/models/config/appointment/appointment-type.model';
import { TranslateService } from '@ngx-translate/core';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-appointment-type-autocomplete',
  standalone: true,
  imports: [FormsModule, AutoComplete],
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
export class AppointmentTypeAutoComplete implements OnInit {
  @Input() value: number | null = null;
  @Input() dropdown = true;
  @Input() showClear = true;
  @Input() appendTo: any = 'body';
  @Input() emptyFilterMessage = '';
  @Input() activeOnly: boolean = true;
  @Output() valueChange = new EventEmitter<number | null>();

  selectedItem: AppointmentTypeModel | null = null;
  filteredItems = signal<AppointmentTypeModel[]>([]);
  private translate = inject(TranslateService);

  constructor(private appointmentTypeCache: AppointmentTypeCacheService) {}

  async ngOnInit() {
    if (!this.emptyFilterMessage) {
      this.emptyFilterMessage = this.translate.instant('common.label.no_results');
    }
    await this.appointmentTypeCache.loadAppointmentTypes();
    this.selectedItem = this.value ? this.appointmentTypeCache.getById(this.value) || null : null;
  }

  filterItems(event: any) {
    const query = (event.query || '').toLowerCase();
    let items = this.getFilteredItems();

    if (!query) {
      if (this.selectedItem && !items.find((item) => item.id === this.selectedItem!.id)) {
        items = [this.selectedItem, ...items];
      }
      this.filteredItems.set(items.slice(0, 100)); // Giới hạn 100 items khi dropdown
      return;
    }

    const filtered = items.filter((item) => item.name && item.name.toLowerCase().includes(query));

    if (
      this.selectedItem &&
      this.selectedItem!.name?.toLowerCase().includes(query) &&
      !filtered.find((item) => item.id === this.selectedItem!.id)
    ) {
      filtered.unshift(this.selectedItem);
    }

    this.filteredItems.set(filtered);
  }

  onSelect(event: any) {
    const value = event.value?.id || null;
    this.value = value;
    this.valueChange.emit(value);
  }

  onClear() {
    this.selectedItem = null;
    this.value = null;
    this.valueChange.emit(null);
  }

  onBlur() {
    if (typeof this.selectedItem === 'string' || (this.selectedItem && !this.selectedItem.id)) {
      this.selectedItem = null;
    }
  }

  private getFilteredItems(): AppointmentTypeModel[] {
    let items = this.appointmentTypeCache.getAll();

    if (this.activeOnly) {
      items = items.filter((item) => item.status === 1);
    }

    return items;
  }
}
