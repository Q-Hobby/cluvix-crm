import {
  ChangeDetectionStrategy,
  AfterViewInit,
  ChangeDetectorRef,
  Component,
  forwardRef,
  inject,
  Input,
  OnChanges,
  SimpleChanges,
  ViewChild,
} from '@angular/core';
import { ControlValueAccessor, NG_VALUE_ACCESSOR } from '@angular/forms';
import { UserFormControl } from '../user-form-control';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-department-user-form-control',
  standalone: true,
  imports: [UserFormControl],
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => DepartmentUserFormControl),
      multi: true,
    },
  ],
  template: `
    <app-user-form-control
      #userControl
      [activeOnly]="activeOnly"
      [department]="department"
      [branchId]="branchId"
      [ariaLabel]="ariaLabel"
    >
    </app-user-form-control>
  `,
})
export class DepartmentUserFormControl implements ControlValueAccessor, AfterViewInit, OnChanges {
  @Input() department!: number;
  @Input() activeOnly: boolean = false;
  @Input() branchId?: number;
  /** Forward xuống `UserFormControl` — xem giải thích ở input cùng tên bên đó (B.19b). */
  @Input() ariaLabel?: string;

  @ViewChild('userControl') userControl!: UserFormControl;
  private pendingValue: any = null;
  private cdr = inject(ChangeDetectorRef);

  ngAfterViewInit() {
    if (this.userControl) {
      this.userControl.registerOnChange((value: any) => {
        this.onChange(value);
      });
      this.userControl.registerOnTouched(() => {
        this.onTouched();
      });

      // Nếu có pending value thì set trong next tick để tránh ExpressionChangedAfterItHasBeenCheckedError
      if (this.pendingValue !== null) {
        setTimeout(() => {
          this.userControl.writeValue(this.pendingValue);
          this.pendingValue = null;
          this.cdr.markForCheck();
        });
      }
    }
  }

  ngOnChanges(changes: SimpleChanges) {
    // Khi branchId thay đổi, force update UserFormControl
    if (changes['branchId'] && this.userControl) {
      // Trigger re-evaluation của computed signal trong UserFormControl
      this.userControl.ngOnChanges(changes);
    }
  }

  writeValue(value: any): void {
    if (this.userControl) {
      this.userControl.writeValue(value);
    } else {
      // Lưu lại để set sau khi ViewChild ready
      this.pendingValue = value;
    }
  }

  registerOnChange(fn: any): void {
    this.onChange = fn;
  }

  registerOnTouched(fn: any): void {
    this.onTouched = fn;
  }

  private onChange = (value: any) => {};

  private onTouched = () => {};
}
