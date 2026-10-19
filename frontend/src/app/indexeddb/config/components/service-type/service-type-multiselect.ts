import {
  ChangeDetectionStrategy,
  Component,
  EventEmitter,
  inject,
  Input,
  OnChanges,
  OnInit,
  Output,
  signal,
  SimpleChanges,
} from '@angular/core';
import { FormsModule } from '@angular/forms';
import { MultiSelect } from 'primeng/multiselect';
import { ServiceTypeCacheService } from './service-type-cache.service';
import { calculateScrollHeight } from '@/indexeddb/config/components/base/scroll-height.util';
import { TranslateService } from '@ngx-translate/core';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-service-type-multiselect',
  standalone: true,
  imports: [FormsModule, MultiSelect],
  template: `
    <p-multiselect
      [options]="serviceTypeOptions()"
      [(ngModel)]="value"
      (onChange)="onValueChange($event)"
      optionLabel="label"
      optionValue="value"
      [filter]="true"
      appendTo="body"
      [resetFilterOnHide]="true"
      [emptyFilterMessage]="emptyMessage"
      [maxSelectedLabels]="2"
      [virtualScroll]="serviceTypeOptions().length > 20"
      [virtualScrollItemSize]="38"
      [scrollHeight]="calculateScrollHeight(serviceTypeOptions().length)"
      [showClear]="true"
      filterBy="label"
      class="w-full"
    />
  `,
})
export class ServiceTypeMultiselect implements OnInit, OnChanges {
  @Input() activeOnly: boolean = false;
  @Output() valueChange = new EventEmitter<any>();

  value: number[] = [];
  serviceTypeOptions = signal<{ value: number; label: string }[]>([]);
  protected readonly calculateScrollHeight = calculateScrollHeight;
  protected emptyMessage = '';
  private translate = inject(TranslateService);

  constructor(private serviceTypeCache: ServiceTypeCacheService) {}

  async ngOnInit() {
    this.emptyMessage = this.translate.instant('common.label.no_results');
    await this.serviceTypeCache.ensureServiceTypesLoaded();
    this.loadServiceTypes();
  }

  ngOnChanges(changes: SimpleChanges) {
    const relevantChanges = ['activeOnly'];
    const hasRelevantChange = relevantChanges.some(
      (key) => changes[key] && !changes[key].firstChange,
    );

    if (hasRelevantChange) {
      this.loadServiceTypes();
    }
  }

  onValueChange(event: any) {
    this.valueChange.emit(event);
  }

  private loadServiceTypes() {
    const filtered = this.serviceTypeCache.filterServiceTypes({
      activeOnly: this.activeOnly,
      includeServiceTypeIds: this.value || [],
    });

    // Đảm bảo selected service types luôn có trong danh sách
    let finalServiceTypes = filtered;

    if (this.value && this.value.length > 0) {
      const allServiceTypes = this.serviceTypeCache.getAllServiceTypes();
      const selectedServiceTypes = allServiceTypes.filter((st) => this.value.includes(st.id!));

      const selectedIds = selectedServiceTypes.map((st) => st.id);
      const nonSelectedServiceTypes = filtered.filter((st) => !selectedIds.includes(st.id));

      finalServiceTypes = [...selectedServiceTypes, ...nonSelectedServiceTypes];
    }

    this.serviceTypeOptions.set(finalServiceTypes.map((s) => ({ value: s.id!, label: s.name! })));
  }
}
