import { computed, DestroyRef, Directive, inject, Input, OnDestroy, OnInit, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { AbstractControl, FormGroup } from '@angular/forms';
import { DynamicDialogConfig } from 'primeng/dynamicdialog';
import { finalize } from 'rxjs';

import { BaseApp } from '@/share/base/BaseApp';
import { BaseApiService } from '@/share/services/base-api.service';
import { CustomValidationService } from '@/share/services/custom-validation.service';
import { UtilsService } from '@/share/services/utils.service';

import { authUser } from '@/core/services/auth';
import { UserCacheService } from '@/indexeddb/config/components/user';

@Directive()
export abstract class BaseForm<T extends { id?: number }>
  extends BaseApp
  implements OnInit, OnDestroy
{
  @Input() initialValue;
  rfModel!: FormGroup;
  abstract modelService: BaseApiService<T>;
  readonly blockSpace: RegExp = /[^\s]/;
  readonly id = signal<number | null>(null);
  readonly model = signal<T>({} as T);
  readonly isToFormData = signal(false);
  readonly sendRaw = signal(false);
  readonly clone = signal<T>({} as T);
  readonly isEditMode = computed(() => !!this.model()?.id);
  canEditBranch = signal(true);
  readonly currentBranchId = signal<number | undefined>(undefined);
  protected submitted = signal(false);
  protected destroyRef = inject(DestroyRef);
  protected customValidationService: CustomValidationService = inject(CustomValidationService);
  protected utilsService: UtilsService = inject(UtilsService);
  protected dialogConfig: DynamicDialogConfig | null = inject(DynamicDialogConfig, {
    optional: true,
  });
  protected userCacheService = inject(UserCacheService);

  constructor() {
    super();
    const idParam = this.dialogConfig?.data?.id;
    this.id.set(idParam);
    const modelParam = structuredClone(this.dialogConfig?.data?.model);
    this.model.set(modelParam);
    const cloneParam = structuredClone(this.dialogConfig?.data?.clone);
    this.clone.set(cloneParam);
  }

  get f() {
    return this.rfModel.controls;
  }

  ngOnInit() {
    if (this.id()) {
      this.modelService.get(this.id()!, {}).subscribe((model: any) => {
        this.initialForm(model);
      });
    } else if (this.model()) {
      this.initialForm(this.model());
    } else {
      //Khởi tạo form bằng obj default
      const obj = { ...this.modelService.getInitialObject(), ...this.initialValue };
      this.initialForm(obj);
    }
  }

  /**
   * Khởi tạo form
   * @param model
   */
  initialForm(model: T | null | undefined): void {
    const validModel = model ?? { ...this.modelService.getInitialObject(), ...this.initialValue };
    this.model.set(validModel);
    this.buildForm(validModel);
    if (!this.rfModel) throw new Error('rfModel must be assigned inside buildForm()');
    this.setupFormValueTracking();
  }

  /**
   * Submit data lên server
   */
  onSubmit(): void {
    if (this.isSubmitting()) return;

    this.submitted.set(true);
    this.markAllTouched();
    if (this.rfModel.invalid) return;

    this.isSubmitting.set(true);
    let responseData: any = null;

    const handleSuccess = (res) => {
      responseData = res;
      this.noti.success( this.translate.instant('common.label.notification'), this.isEditMode() ? (res.message || this.translate.instant('common.message.update_success')) : this.translate.instant('common.message.create_success'));
      // Cảnh báo mềm từ BE (lưu thành công nhưng cần lưu ý — vd lương < mức tối thiểu vùng)
      if (res?.warning) this.noti.warnMsg(res.warning);
    };

    const handleFinalize = () => {
      this.isSubmitting.set(false);
      if (responseData) {
        this.dialogRef.close(this.isEditMode() ? (responseData.data || responseData) : responseData);
      }
    };

    const payload = this.getPayload();

    if (this.isEditMode() && !this.clone()) {
      this.modelService
        .update(this.model().id!, payload, {}, this.isToFormData())
        .pipe(finalize(handleFinalize))
        .subscribe({
          next: handleSuccess,
        });
    } else {
      this.modelService
        .create(payload, {}, this.isToFormData())
        .pipe(finalize(handleFinalize))
        .subscribe({
          next: handleSuccess,
        });
    }
  }

  /**
   * Close form
   * @param event
   */
  closeForm(event: Event) {
    if (this.rfModel.dirty) {
      this.confirmationService.confirm({
        target: (event.target ?? undefined),
        message: this.translate.instant('common.message.confirm_cancel'),
        icon: 'pi pi-exclamation-triangle',
        acceptButtonProps: {severity: 'danger', size: 'small'},
        accept: () => {
          this.rfModel.reset({}, { emitEvent: false });
          this.dialogRef.close(false);
        },
      });
    } else {
      this.rfModel.reset({}, { emitEvent: false });
      this.dialogRef.close(false);
    }
  }

  /**
   * format date
   * @param input
   */
  formatDate(input: any) {
    const day = input.slice(0, 2);
    const month = input.slice(2, 4);
    const year = input.slice(4, 8);
    return `${day}/${month}/${year}`;
  }

  /**
   * Xử lý date nếu nhập bằng tay
   * @param $event
   * @param selectedDate
   */
  handleSelectedDate($event: any, selectedDate: any) {
    if ($event.keyCode === 13) {
      if (!selectedDate.includes('/')) {
        selectedDate = this.formatDate(selectedDate);
      }
      const [day, month, year] = selectedDate.split('/');
      if (year && +month <= 12 && +day <= 31) {
        const finalDate = new Date(+year, +month - 1, +day);
        this.rfModel.patchValue({
          [$event.target.getAttribute('formControlName')]: finalDate,
        });
        return finalDate;
      } else {
        return '';
      }
    }
  }

  /**
   * Chắc chắn date được đưa về đúng định dạng
   * @param form
   * @param field
   */
  normalizeDateField(form: FormGroup, field: string) {
    const control = form.get(field);
    if (!control) return;

    const val = control.value;
    if (!val) {
      control.setValue(this.timezoneService.now());
    } else if (typeof val === 'number') {
      // Unix timestamp (seconds hoặc ms)
      control.setValue(new Date(val < 1e12 ? val * 1000 : val));
    } else if (typeof val === 'string') {
      control.setValue(new Date(val));
    }
  }

  ngOnDestroy(): void {
    if (this.dialogRef) {
      this.dialogRef.close(false);
    }
  }

  /**
   * Override để transform payload trước khi submit
   */
  protected getPayload(): any {
    return this.sendRaw() ? this.rfModel.getRawValue() : this.rfModel.value;
  }

  protected abstract buildForm(model: T);

  protected markAllTouched(): void {
    Object.values(this.rfModel?.controls ?? {}).forEach(c => c.markAsTouched());
  }

  protected isFieldInvalid(field: string): boolean {
    const c = this.rfModel?.get(field);
    return !!c && c.invalid;
  }

  protected isGroupFieldInvalid(group: AbstractControl | null | undefined, field: string): boolean {
    const c = group?.get(field);
    return !!c && c.invalid;
  }

  protected getError(field: string): string | null {
    const c = this.rfModel?.get(field);
    if (!c?.errors || !this.isFieldInvalid(field)) return null;
    if (c.errors['required']) return this.translate.instant('common.validation.required');
    if (c.errors['email']) return this.translate.instant('common.validation.email');
    if (c.errors['minlength']) return this.translate.instant('common.validation.minlength', { min: c.errors['minlength'].requiredLength });
    if (c.errors['maxlength']) return this.translate.instant('common.validation.maxlength', { max: c.errors['maxlength'].requiredLength });
    if (c.errors['pattern']) return this.translate.instant('common.validation.pattern');
    return this.translate.instant('common.validation.invalid');
  }

  protected isEditableOnUpdate(branchId: any): boolean {
    // Create: luôn cho chọn
    if (!this.isEditMode()) return true;

    // Update: nếu chưa có branch => vẫn cho chọn
    if (branchId == null || branchId === '') return true;

    // So sánh trong quyền user
    const group = (authUser()?.branchId ?? []).map((x: any) => +x);
    return group.includes(+branchId);
  }

  /**
   * Giữ lại lựa chọn cũ (single-select bác sĩ/KTV...) nếu người đó vẫn thuộc
   * branch/department mới; chỉ clear khi không còn hợp lệ. Dùng trong subscribe
   * branch_id.valueChanges thay vì clear cứng — tránh mất default đã setup.
   */
  protected keepValidSingleUser(
    control: string | AbstractControl | null,
    branchId: number | null | undefined,
    department: number | undefined,
  ): void {
    const targetControl = typeof control === 'string' ? this.rfModel.controls[control] : control;
    const currentId = targetControl?.value;
    if (!currentId) return;

    const validIds = new Set(
      this.userCacheService
        .filterUsers({
          department,
          branchId: branchId ?? undefined,
          activeOnly: !this.isEditMode(),
        })
        .map((u) => u.id),
    );
    if (!validIds.has(currentId)) {
      targetControl?.setValue(null);
    }
  }

  /**
   * Bản multiselect của keepValidSingleUser — giữ các id vẫn hợp lệ, chỉ lọc
   * bỏ những id không còn thuộc branch/department mới.
   */
  protected keepValidUserList(
    controlName: string,
    branchId: number | null | undefined,
    department: number | undefined,
  ): void {
    const control = this.rfModel.controls[controlName];
    const current = control.value as number[] | null;
    if (!current || current.length === 0) return;

    const validIds = new Set(
      this.userCacheService
        .filterUsers({
          department,
          branchId: branchId ?? undefined,
          activeOnly: !this.isEditMode(),
        })
        .map((u) => u.id),
    );
    const filtered = current.filter((id) => validIds.has(id));
    if (filtered.length !== current.length) {
      control.setValue(filtered);
    }
  }

  protected setupFormValueTracking() {
    // Track branch_id changes
    this.rfModel.get('branch_id')?.valueChanges
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe((value) => {
        this.currentBranchId.set(value);
      });

    // Set initial values
    this.currentBranchId.set(this.rfModel.get('branch_id')?.value ?? undefined);
  }
}
