import {
  ChangeDetectionStrategy,
  AfterViewInit,
  Component,
  forwardRef,
  Input,
  OnChanges,
  SimpleChanges,
  ViewChild,
} from '@angular/core';
import { ControlValueAccessor, NG_VALUE_ACCESSOR } from '@angular/forms';
import { UserMultiSelectFormControl } from '@/indexeddb/config/components/user';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-department-user-multiselect',
  standalone: true,
  imports: [UserMultiSelectFormControl],
  providers: [
    {
      provide: NG_VALUE_ACCESSOR,
      useExisting: forwardRef(() => DepartmentUserMultiselect),
      multi: true,
    },
  ],
  template: `
    <app-user-multiselect-form-control
      #userControl
      [activeOnly]="activeOnly"
      [department]="department"
      [branchId]="branchId"
    >
    </app-user-multiselect-form-control>
  `,
})
export class DepartmentUserMultiselect implements ControlValueAccessor, AfterViewInit, OnChanges {
  @Input() department!: number;
  @Input() activeOnly: boolean = false;
  @Input() branchId?: number;
  @Input() appendTo: any = 'body';
  @Input() emptyFilterMessage: string = '';
  @Input() resetFilterOnHide: boolean = true;

  @ViewChild('userControl') userControl!: UserMultiSelectFormControl;
  private pendingValue: any = null;

  ngAfterViewInit() {
    if (this.userControl) {
      this.userControl.registerOnChange((value: any) => {
        this.onChange(value);
      });
      this.userControl.registerOnTouched(() => {
        this.onTouched();
      });

      if (this.pendingValue !== null) {
        this.userControl.writeValue(this.pendingValue);
        this.pendingValue = null;
      }
    }
  }

  ngOnChanges(changes: SimpleChanges) {
    if (changes['department'] || changes['branchId']) {
      // Inputs changed, UserMultiSelectFormControl will handle automatically
    }
  }

  writeValue(value: any): void {
    if (this.userControl) {
      this.userControl.writeValue(value);
    } else {
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
