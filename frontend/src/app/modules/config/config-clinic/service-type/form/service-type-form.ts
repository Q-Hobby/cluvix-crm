import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { FormField, required, SchemaPathTree } from '@angular/forms/signals';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { finalize } from 'rxjs';

import { ServiceTypeModel } from '@/core/models/config/clinic/service-type.model';
import { ServiceTypeService } from '@/core/services/config/clinic/service-type.service';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import { FormShareImports } from '@/share/import/form-share-imports';

/**
 * Signal Forms cần KEY tồn tại thật trên object model (`rootField.x` trả `undefined`, không phải
 * `FieldTree`, nếu key thiếu hoặc giá trị `undefined` — gotcha #1 `angular22.md`). `getInitialObject()`/
 * response BE (Go `omitempty`) có thể thiếu key cho field rỗng — merge qua template này TRƯỚC khi set
 * vào `model`, cả create lẫn edit mode.
 *
 * `name` bind `<input pInputText type="text">` → seed `''` (gotcha #9), KHÔNG `null` (Signal
 * Forms coi field số nếu seed `null`, gõ chữ lỗi `parse`).
 */
function emptyServiceTypeModel(): ServiceTypeModel {
  return {
    name: '',
    status: null,
    emr_order_type: null,
  } as unknown as ServiceTypeModel;
}

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-service-type-form',
  templateUrl: './service-type-form.html',
  styles: [],
  imports: [...FormShareImports, FormField],
  standalone: true,
})
export class ServiceTypeForm extends BaseSignalForm<ServiceTypeModel> {
  modelService = inject(ServiceTypeService);

  override ngOnInit(): void {
    const dialogModel = this.dialogConfig?.data?.model as ServiceTypeModel | undefined;
    if (this.id()) {
      this.model.set({ ...this.modelService.getInitialObject() } as unknown as ServiceTypeModel);
      this.modelService
        .get(this.id()!, {})
        .pipe(takeUntilDestroyed(this.destroyRef))
        .subscribe((loaded: any) => this._applyModel({ ...this.modelService.getInitialObject(), ...loaded }));
    } else if (dialogModel) {
      this._applyModel(dialogModel);
    } else {
      this._applyModel({ ...this.modelService.getInitialObject(), ...this.initialValue } as unknown as ServiceTypeModel);
    }
  }

  /** Chuẩn hoá model trước khi set vào FieldTree — seed đủ key. */
  private _applyModel(model: ServiceTypeModel): void {
    const merged = { ...emptyServiceTypeModel(), ...model };

    this.model.set(merged);
  }

  /**
   * `ServiceTypeService` có `createServiceType()`/`updateServiceType()` riêng (không phải
   * `create()`/`update()` chung của `BaseApiService`) — phải override thay vì dùng
   * `BaseSignalForm.onSubmit()` chung.
   */
  override onSubmit() {
    if (this.isSubmitting()) return;
    this.submitted.set(true);
    this.rootField().markAsTouched();

    if (this.rootField().invalid()) return;

    this.isSubmitting.set(true);

    const formValue: any = this.getPayload();

    if (this.isEditMode() && !this.clone()) {
      this.modelService
        .updateServiceType(this.model().id!, formValue)
        .pipe(finalize(() => this.isSubmitting.set(false)))
        .subscribe({
          next: (res: any) => {
            this.noti.successRes(res);
            this.dialogRef.close(res.data);
          },
          error: () => {},
        });
    } else {
      this.modelService
        .createServiceType(formValue)
        .pipe(finalize(() => this.isSubmitting.set(false)))
        .subscribe({
          next: (res: any) => {
            this.noti.successRes(res);
            this.dialogRef.close(res.data);
          },
          error: () => {},
        });
    }
  }

  protected override buildSchema(p: SchemaPathTree<ServiceTypeModel>): void {
    required(p.name!);
  }
}
