import { ChangeDetectionStrategy, ChangeDetectorRef, Component, inject, OnInit, TrackByFunction } from '@angular/core';
import { FormField, required, requiredError, SchemaPathTree, validate } from '@angular/forms/signals';
import { FormShareImports } from '@/share/import/form-share-imports';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import { TAG_SEVERITY_OPTION } from '@/define';
import { CustomerRankServicesModel } from '@/core/models/config/customer/customer-rank-services.model';
import { CustomerRankServicesService } from '@/core/services/config/customer/customer-rank-services.service';
import { CustomerRanksAutocompleteFormControl } from '@/indexeddb/config/components/customer-ranks';
import { ServicesHelperService } from '@/indexeddb/config/components/services/services-helper.service';
import { PickListModule } from 'primeng/picklist';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-customer-rank-services-form',
  templateUrl: './customer-rank-services-form.html',
  styleUrl: './customer-rank-services-form.scss',
  imports: [...FormShareImports, FormField, CustomerRanksAutocompleteFormControl, PickListModule],
  standalone: true,
})
export class CustomerRankServicesForm
  extends BaseSignalForm<CustomerRankServicesModel>
  implements OnInit
{
  modelService = inject(CustomerRankServicesService);
  // Khởi tạo với placeholder để giữ height ổn định
  availableServices: any[] = [];
  selectedServices: any[] = [];
  isLoadingServices = true; // Loading state
  protected readonly TAG_SEVERITY_OPTION = TAG_SEVERITY_OPTION;
  private servicesHelper = inject(ServicesHelperService);
  private cdr = inject(ChangeDetectorRef);
  // Performance optimization: Set cache cho O(1) lookup
  private selectedIdsSet = new Set<number>();

  // TrackBy function cho ngFor optimization
  trackByServiceId: TrackByFunction<any> = (index: number, item: any) => item?.id || index;

  /**
   * KHÔNG có FormArray thật (xem Dev Notes) — `p-picklist` tự quản lý 2 mảng component state
   * (availableServices/selectedServices), chỉ đồng bộ ngược 1 field đơn `service_ids: number[]` vào
   * `rootField`. Override để load danh mục dịch vụ SAU khi `model` đã seed đủ (cả 2 nhánh create/update
   * của `BaseSignalForm.ngOnInit()` đều set `model` ĐỒNG BỘ — form này không bao giờ truyền `id` qua
   * dialog data, chỉ `model` cho update — nên `this.rootField.service_ids!().value()` đọc đúng ngay
   * sau `super.ngOnInit()`).
   */
  override ngOnInit() {
    super.ngOnInit();

    void this.servicesHelper
      .ensureServicesLoaded()
      .then(() => {
        this.loadServices();
        this.cdr.markForCheck();
      })
      .catch((error) => {
        console.error('Error loading services:', error);
        this.isLoadingServices = false;
        this.cdr.markForCheck();
      });
  }

  protected override buildSchema(p: SchemaPathTree<CustomerRankServicesModel>): void {
    required(p.rank_id!);
    required(p.discount_percent!);
    // `required()` không coi mảng rỗng `[]` là "rỗng" (gotcha #10 angular22.md) — dùng `validate()`
    // thủ công để giữ đúng ngữ nghĩa "phải chọn ít nhất 1 dịch vụ" như `Validators.required` bản gốc.
    validate(p.service_ids!, ({ value }) => ((value()?.length ?? 0) === 0 ? requiredError() : undefined));
  }

  loadServices() {
    try {
      const selectedIds = this.rootField.service_ids!().value() || [];

      // Update Set cache for O(1) lookup
      this.selectedIdsSet = new Set(selectedIds);

      const { available, selected } = this.servicesHelper.getServicesForPickList(selectedIds);

      this.availableServices = available;
      this.selectedServices = selected;

      // Tắt loading state
      this.isLoadingServices = false;
    } catch (error) {
      console.error('Error loading services for picklist:', error);
      this.availableServices = [];
      this.selectedServices = [];
      this.isLoadingServices = false;
    }
  }

  onServicesChange() {
    const serviceIds = this.selectedServices.map((s) => s.id).filter((id) => id != null);

    // Update Set cache
    this.selectedIdsSet = new Set(serviceIds);

    this.rootField.service_ids!().value.set(serviceIds);
  }

  // Handle single item move events
  onMoveToTarget(event: any) {
    // Manual update arrays để đảm bảo không bị replace
    const movedItems = event.items;
    const movedIds = new Set(movedItems.map((item: any) => item.id));

    // Thêm vào selected (tránh duplicate)
    const newSelected = [...this.selectedServices];
    movedItems.forEach((item: any) => {
      if (!newSelected.some((s) => s.id === item.id)) {
        newSelected.push(item);
      }
    });

    // Xóa khỏi available
    const newAvailable = this.availableServices.filter((item) => !movedIds.has(item.id));

    this.selectedServices = newSelected;
    this.availableServices = newAvailable;

    this.onServicesChange();
  }

  onMoveToSource(event: any) {
    // Manual update arrays để đảm bảo không bị replace
    const movedItems = event.items;
    const movedIds = new Set(movedItems.map((item: any) => item.id));

    // Thêm vào available (tránh duplicate)
    const newAvailable = [...this.availableServices];
    movedItems.forEach((item: any) => {
      if (!newAvailable.some((s) => s.id === item.id)) {
        newAvailable.push(item);
      }
    });

    // Xóa khỏi selected
    const newSelected = this.selectedServices.filter((item) => !movedIds.has(item.id));

    this.availableServices = newAvailable;
    this.selectedServices = newSelected;

    this.onServicesChange();
  }

  onMoveAllToTarget(event: any) {
    // Optimized: Use Set for O(1) lookup instead of some() O(n)
    const movedIds = new Set(event.items.map((item: any) => item.id));

    this.selectedServices = [...this.selectedServices, ...event.items];
    this.availableServices = this.availableServices.filter((item) => !movedIds.has(item.id));

    this.onServicesChange();
  }

  onMoveAllToSource(event: any) {
    // Optimized: Use Set for O(1) lookup instead of some() O(n)
    const movedIds = new Set(event.items.map((item: any) => item.id));

    this.availableServices = [...this.availableServices, ...event.items];
    this.selectedServices = this.selectedServices.filter((item) => !movedIds.has(item.id));

    this.onServicesChange();
  }
}
