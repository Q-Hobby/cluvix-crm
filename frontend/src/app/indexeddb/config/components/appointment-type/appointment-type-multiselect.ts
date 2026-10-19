import { ChangeDetectionStrategy, Component, EventEmitter, inject, Input, OnInit, Output, signal } from '@angular/core';

import { FormsModule } from '@angular/forms';
import { MultiSelect } from 'primeng/multiselect';
import { AppointmentTypeCacheService } from './appointment-type-cache.service';
import { calculateScrollHeight } from '@/indexeddb/config/components/base/scroll-height.util';
import { AppointmentTypeModel } from '@/core/models/config/appointment/appointment-type.model';
import { TranslateService } from '@ngx-translate/core';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-appointment-type-multiselect',
  standalone: true,
  imports: [FormsModule, MultiSelect],
  template: `
    <p-multiselect
      [(ngModel)]="value"
      [options]="appointmentTypeOptions()"
      optionLabel="name"
      optionValue="id"
      [showClear]="showClear"
      [filter]="filter"
      filterBy="name"
      [appendTo]="appendTo"
      [emptyFilterMessage]="emptyFilterMessage"
      [virtualScroll]="virtualScroll"
      [virtualScrollItemSize]="virtualScrollItemSize"
      [maxSelectedLabels]="2"
      [scrollHeight]="calculateScrollHeight(appointmentTypeOptions().length)"
      (onChange)="onSelectionChange($event)"
      class="w-full"
    >
    </p-multiselect>
  `,
})
export class AppointmentTypeMultiSelect implements OnInit {
  @Input() value: number[] = [];
  @Input() showClear = true;
  @Input() activeOnly: boolean = false;
  @Input() appendTo: any = 'body';
  @Input() filter: boolean = true;
  @Input() virtualScroll: boolean = true;
  @Input() virtualScrollItemSize: number = 38;
  @Input() emptyFilterMessage: string = '';
  @Output() valueChange = new EventEmitter<number[]>();

  appointmentTypeOptions = signal<AppointmentTypeModel[]>([]);
  protected readonly calculateScrollHeight = calculateScrollHeight;
  private translate = inject(TranslateService);

  constructor(private appointmentTypeCache: AppointmentTypeCacheService) {}

  async ngOnInit() {
    if (!this.emptyFilterMessage) {
      this.emptyFilterMessage = this.translate.instant('common.label.no_results');
    }
    await this.appointmentTypeCache.loadAppointmentTypes();
    this.loadAppointmentTypes();
  }

  onSelectionChange(event: any) {
    this.valueChange.emit(event.value);
  }

  private loadAppointmentTypes() {
    let filteredTypes = this.appointmentTypeCache.getAll();

    if (this.activeOnly) {
      filteredTypes = filteredTypes.filter((type) => type.status === 1);
    }

    // Đảm bảo selected items luôn ở đầu danh sách
    const selectedTypes = filteredTypes.filter((t) => this.value?.includes(t.id!));
    const unselectedTypes = filteredTypes.filter((t) => !this.value?.includes(t.id!));
    const sortedTypes = [...selectedTypes, ...unselectedTypes];

    this.appointmentTypeOptions.set(sortedTypes);
  }
}
