import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import {
  emailError,
  FormField,
  maxLength,
  minLength,
  required,
  SchemaPath,
  SchemaPathTree,
  validate,
} from '@angular/forms/signals';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { finalize } from 'rxjs';

import { BranchModel } from '@/core/models/config/system/branch.model';
import { BranchService } from '@/core/services/config/system/branch.service';
import { BaseSignalForm } from '@/share/base/base-signal-form';
import { FormShareImports } from '@/share/import/form-share-imports';
import { UtilsService } from '@/share/services/utils.service';
import { authUser } from '@/core/services/auth/auth.state';

/**
 * Signal Forms cần KEY tồn tại thật trên object model (`rootField.x` trả `undefined`, không phải
 * `FieldTree`, nếu key thiếu hoặc giá trị `undefined` — gotcha #1 `angular22.md`). `getInitialObject()`/
 * response BE (Go `omitempty`) có thể thiếu key cho field rỗng — merge qua template này TRƯỚC khi set
 * vào `model`, cả create lẫn edit mode.
 *
 * `name`/`code`/`hotline`/`email` bind `<input pInputText type="text">` → seed `''` (gotcha #9), KHÔNG
 * `null` (Signal Forms coi field số nếu seed `null`, gõ chữ lỗi `parse`). `address` bind
 * `<textarea pTextarea>` → seed `null` bình thường (KHÔNG dính gotcha #9).
 */
function emptyBranchModel(): BranchModel {
  return {
    name: '',
    code: '',
    address: null,
    hotline: '',
    email: '',
    statusOpenAppointment: null,
    status: null,
  } as unknown as BranchModel;
}

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-branch-form',
  templateUrl: './branch-form.html',
  styles: [''],
  imports: [...FormShareImports, FormField],
  standalone: true,
})
export class BranchForm extends BaseSignalForm<BranchModel> {
  modelService = inject(BranchService);
  private utilsService = inject(UtilsService);
  // Giữ nguyên logic gốc (`BaseForm.blockSpace`) — chặn ký tự khoảng trắng khi gõ code/email/hotline.
  protected readonly blockSpace: RegExp = /[^\s]/;

  override ngOnInit(): void {
    const dialogModel = this.dialogConfig?.data?.model as BranchModel | undefined;
    if (this.id()) {
      this.model.set({ ...this.modelService.getInitialObject() } as unknown as BranchModel);
      this.modelService
        .get(this.id()!, {})
        .pipe(takeUntilDestroyed(this.destroyRef))
        .subscribe((loaded: any) => this._applyModel({ ...this.modelService.getInitialObject(), ...loaded }));
    } else if (dialogModel) {
      this._applyModel(dialogModel);
    } else {
      this._applyModel({ ...this.modelService.getInitialObject(), ...this.initialValue } as unknown as BranchModel);
    }
  }

  /** Chuẩn hoá model trước khi set vào FieldTree — seed đủ key. */
  private _applyModel(model: BranchModel): void {
    this.model.set({
      ...emptyBranchModel(),
      ...model,
    });
  }

  /**
   * `BranchService` có `createBranch()`/`updateBranch()` riêng (không phải `create()`/`update()` chung
   * của `BaseApiService`) — phải override thay vì dùng `BaseSignalForm.onSubmit()` chung.
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
        .updateBranch(this.model().id!, formValue, {})
        .pipe(finalize(() => this.isSubmitting.set(false)))
        .subscribe({
          next: (res: any) => {
            sessionStorage.setItem('post_reload_toast', 'config.branch.update_success');
            this.dialogRef.close(res.data);
          },
          error: () => {},
        });
    } else {
      this.modelService
        .createBranch(formValue, {})
        .pipe(finalize(() => this.isSubmitting.set(false)))
        .subscribe({
          next: (res: any) => {
            const newId: number = res?.data?.id;
            if (newId) {
              const current = authUser();
              if (current && !current.branchId?.includes(newId)) {
                const updated = { ...current, branchId: [...(current.branchId ?? []), newId] };
                authUser.set(updated);
                this.storageService.saveStorage('auth_user', updated);
              }
            }
            sessionStorage.setItem('post_reload_toast', 'config.branch.create_success');
            this.dialogRef.close(res.data);
          },
          error: () => {},
        });
    }
  }

  protected override buildSchema(p: SchemaPathTree<BranchModel>): void {
    required(p.name!);
    required(p.code!);
    required(p.address!);
    minLength(p.hotline! as SchemaPath<string>, 10);
    maxLength(p.hotline! as SchemaPath<string>, 12);
    validate(p.email! as SchemaPath<string>, ({ value }) => {
      const v = value();
      return !v || this.utilsService.isEmail(v) ? null : emailError();
    });
  }
}
