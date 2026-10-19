import { ChangeDetectionStrategy, Component, DestroyRef, inject, OnInit, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Validators } from '@angular/forms';
import { catchError, finalize, of } from 'rxjs';
import { DynamicDialogConfig } from 'primeng/dynamicdialog';

import { BaseApp } from '@/share/base/BaseApp';
import { FormShareImports } from '@/share/import/form-share-imports';
import { OrderPlanService } from '@/core/services/order-plan.service';
import { ServicesHelperService } from '@/indexeddb/config/components/services';

/**
 * Dialog "Đổi dịch vụ" — order-service-change story-06.
 * Tách biệt khỏi order-plan-form: đổi service_id của 1 order_service_used ĐÃ có payment
 * là hành động riêng (transaction/side-effect khác hẳn update field thông thường), KHÔNG
 * bật lại control service_id inline trong form OSU.
 */
@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-change-service-dialog',
  templateUrl: './change-service-dialog.html',
  standalone: true,
  imports: [...FormShareImports],
})
export class ChangeServiceDialog extends BaseApp implements OnInit {
  private dialogConfig = inject(DynamicDialogConfig);
  private destroyRef = inject(DestroyRef);
  private orderPlanService = inject(OrderPlanService);
  private servicesHelper = inject(ServicesHelperService);

  private orderServiceUsedId!: number;
  private updatedAt!: number;

  readonly serviceOptions = signal<{ value: number; label: string }[]>([]);
  rfModel = this.fb.group({
    new_service_id: [null as number | null, [Validators.required]],
    reason: [''],
  });

  ngOnInit(): void {
    const data = this.dialogConfig?.data ?? {};
    this.orderServiceUsedId = data.orderServiceUsedId;
    this.updatedAt = data.updated_at;

    const options = this.servicesHelper
      .getServicesForChange(data.currentServiceId)
      .map((s: any) => ({ value: s.id, label: s.name }));
    this.serviceOptions.set(options);
  }

  closeForm(): void {
    this.dialogRef.close();
  }

  onSubmit(): void {
    if (this.isSubmitting()) return;
    if (this.rfModel.invalid) {
      this.rfModel.markAllAsTouched();
      return;
    }

    this.isSubmitting.set(true);
    const raw = this.rfModel.getRawValue();
    this.orderPlanService
      .changeService(this.orderServiceUsedId, {
        new_service_id: raw.new_service_id!,
        reason: raw.reason || undefined,
        updated_at: this.updatedAt,
      })
      .pipe(
        takeUntilDestroyed(this.destroyRef),
        finalize(() => this.isSubmitting.set(false)),
        catchError(() => of(null)),
      )
      .subscribe((res) => {
        if (res) {
          this.noti.successMsg(this.translate.instant('crm.order_plan.change_service.success'));
          this.dialogRef.close(res.data);
        }
      });
  }
}
