import { ChangeDetectionStrategy, Component } from '@angular/core';
import { BaseMultiSelectIndexed } from '../base/base-multiselect-indexed';
import { UserProfileModel } from '@/core/models/user-profile.model';

@Component({
  changeDetection: ChangeDetectionStrategy.OnPush,
  selector: 'app-user-multiselect',
  standalone: true,
  imports: [BaseMultiSelectIndexed],
  template: `
    <app-base-multiselect-indexed
      tableName="users"
      optionLabel="fullname"
      [value]="value"
      (valueChange)="valueChange.emit($event)"
      [activeOnly]="activeOnly"
      [allowedStatuses]="allowedStatuses"
      [appendTo]="appendTo"
      [filter]="filter"
      [resetFilterOnHide]="resetFilterOnHide"
      [virtualScroll]="virtualScroll"
      [virtualScrollItemSize]="virtualScrollItemSize"
      [emptyFilterMessage]="emptyFilterMessage"
    >
    </app-base-multiselect-indexed>
  `,
})
export class UserMultiSelect extends BaseMultiSelectIndexed<UserProfileModel> {
  override tableName = 'users'; // Fix: ngOnInit kế thừa từ base cần tableName thật, không phải giá trị mặc định undefined
  override allowedStatuses = [9, 10];
}
