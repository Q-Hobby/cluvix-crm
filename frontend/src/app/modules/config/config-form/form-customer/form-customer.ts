import { ChangeDetectionStrategy, ChangeDetectorRef, Component, DestroyRef, inject, OnInit } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { AbstractControl, FormArray, FormGroup } from '@angular/forms';

import { ControlFieldInTableService } from '@/core/services/config/customize-form/control-field-in-table.service';
import { BaseApp } from '@/share/base/BaseApp';
import { PERMISSION_SELECT } from '@/define';
import { FormShareImports } from '@/share/import/form-share-imports';
import { finalize, Subject, takeUntil } from 'rxjs';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-config-customize-form-customer',
  templateUrl: './form-customer.html',
  standalone: true,
  imports: [...FormShareImports],
  styleUrls: ['./form-customer.scss'],
})
export class FormCustomer extends BaseApp implements OnInit {
  modelService = inject(ControlFieldInTableService);
  form!: FormGroup;
  plusMega = 128;
  // Dùng cho *hasPerm trong template (BaseList cấp sẵn, BaseApp thì không).
  protected readonly permission = PERMISSION_SELECT;
  private cdr = inject(ChangeDetectorRef);
  // BaseList cấp sẵn destroyRef, BaseApp thì không — khai tại chỗ cho takeUntilDestroyed.
  private destroyRef = inject(DestroyRef);
  private reload$ = new Subject<void>();

  constructor() {
    super();
  }

  /**
   * Lấy FormArray fields
   */
  get fieldsCtr(): FormArray {
    return this.form.controls['fields'] as FormArray;
  }

  /**
   * Load dữ liệu
   */
  // KHÔNG phải list: bảng này sửa tại chỗ, không phân trang, không filter panel.
  // Tải dữ liệu bằng method riêng thay cho vòng đời searchHandler/getList của BaseList.
  async loadFields() {
    this.loadingService.showLoading('customize-form-customer-list');
    this.modelService
      .getFields(this.modelService.CUSTOMIZE_FORM_ID)
      .pipe(
        finalize(() => {
          this.isSubmitting.set(false);
          this.loadingService.hideLoading('customize-form-customer-list');
        }),
      )
      .subscribe({
        next: (res: any) => {
          const fields = res?.data ?? [];
          if (fields.length) {
            this.pathDataFields(fields);
            this.registerValueChanges();
          }
          this.cdr.markForCheck();
        },
      });
  }

  /**
   * Đưa dữ liệu vào FormArray
   * @param fields
   */
  pathDataFields(fields: []) {
    this.reload$.next(); // Hủy subscription của lần load trước, tránh tích lũy
    this.fieldsCtr.clear(); // Xóa dữ liệu cũ
    fields.forEach((field: any) => {
      const group = this.fb.group({
        field_name: [field.field_name || false],
        field_label: [field.field_label || false],
        create: [field.create || false],
        update: [field.update || false],
        edit: [field.edit || false],
      });

      // Lắng nghe sự thay đổi giá trị của 'edit'
      group
        ?.get('edit')
        ?.valueChanges.pipe(takeUntil(this.reload$), takeUntilDestroyed(this.destroyRef))
        .subscribe((isChecked: boolean) => {
          const createControl = group.get('create');
          if (isChecked) {
            // Nếu checkbox edit được check, tự động check 'create'
            createControl?.setValue(true);
          }
        });
      this.fieldsCtr.push(group); // Thêm group vào FormArray
    });
  }

  /**
   * Khởi tạo form sau khi khởi tạo BaseList
   */
  ngOnInit() {
    this.form = this.fb.group({
      fields: this.fb.array([]),
    });
    this.loadFields();
  }

  /**
   * Xử lý disable khi mới tải data
   */
  registerValueChanges() {
    this.fieldsCtr.controls.forEach((control: AbstractControl) => {
      const group = control as FormGroup;
      const createControl = group.get('create');
      const editControl = group.get('edit');
      // Tránh lặp lại emitEvent: false
      const options = { emitEvent: false };

      if (editControl?.value) {
        // edit=true luôn buộc create=true khi lưu → hiển thị khớp ngay từ lúc load
        createControl?.setValue(true, options);
        createControl?.disable(options);
      } else {
        createControl?.enable(options);
      }
    });
  }

  /**
   * Xử lý checkbox Không được cập nhật
   * @param event
   * @param index
   */
  onCheckboxChange(event: any, index: any) {
    const group = this.fieldsCtr.at(index) as FormGroup;
    const createControl = group.get('create');
    if (event.checked) {
      this.fieldsCtr.at(index).get('create')?.disable();
      createControl?.setValue(true);
    } else {
      this.fieldsCtr.at(index).get('create')?.enable();
    }
  }

  /**
   * Lưu dữ liệu
   */
  onSave() {
    if (this.form.invalid) {
      this.noti.errorMsg(this.translate.instant('common.message.form_invalid'));
      return;
    }

    const formValues = this.form.getRawValue().fields;

    const updatedValues = formValues.map((field: any) => ({
      ...field,
      create: field.edit ? true : field.create,
    }));

    this.modelService
      .saveFields(this.modelService.CUSTOMIZE_FORM_ID, { control: updatedValues })
      .subscribe({
        next: (res: any) => {
          this.noti.successRes(res);
          this.loadFields();
        },
        error: (error: any) => {
          this.noti.errorMsg(error?.message || this.translate.instant('common.message.error'));
          console.error(error);
        },
      });
  }

  /**
   * Xử lý làm lại
   * @param event
   */
  reBuild(event: Event) {
    this.confirmationService.confirm({
      target: (event.target ?? undefined),
      message: this.translate.instant('config.form_customer.confirm_reset'),
      icon: 'pi pi-exclamation-triangle',
      acceptButtonProps: {severity: 'danger', size: 'small'},
      accept: () => {
        this.modelService.reBuild(1).subscribe({
          next: (res: any) => {
            this.noti.successRes(res);
            this.loadFields();
          },
        });
      },
      reject: () => {},
    });
  }
}
